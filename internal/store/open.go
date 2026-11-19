package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// Pragmas in the DSN apply to every connection the pool opens. secure_delete
// overwrites deleted rows with zeros, and temp_store keeps the copy that
// VACUUM makes in memory instead of in a temporary file.
const pragmas = "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)" +
	"&_pragma=secure_delete(1)&_pragma=temp_store(2)"

// Path returns the database path. It uses $XDG_DATA_HOME/tabby when
// XDG_DATA_HOME is set to an absolute path, on every platform, and falls back
// to the platform default otherwise. Relative values are ignored, as the XDG
// Base Directory Specification requires. While a database in a location that
// an earlier version used has not been moved yet, it returns that location.
func Path() (string, error) {
	path, err := defaultPath()
	if err != nil {
		return "", err
	}

	if legacy := legacyPath(path); legacy != "" {
		return legacy, nil
	}

	return path, nil
}

func defaultPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "tabby", "tabby.db"), nil
}

func dataDir() (string, error) {
	if dir := absoluteEnv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}

	// On Windows, this is %LocalAppData%, which does not roam with the user
	// profile the way %AppData% does.
	if runtime.GOOS == "windows" {
		return os.UserCacheDir()
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share"), nil
}

func absoluteEnv(name string) string {
	if dir := os.Getenv(name); filepath.IsAbs(dir) {
		return dir
	}

	return ""
}

// legacyPaths returns the database paths that earlier versions used, in the
// order they took precedence.
func legacyPaths() []string {
	var dirs []string
	if dir := absoluteEnv("XDG_CONFIG_HOME"); dir != "" {
		dirs = append(dirs, dir)
	}

	if runtime.GOOS == "windows" {
		if dir := os.Getenv("AppData"); dir != "" {
			dirs = append(dirs, dir)
		}
	} else if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config"))
	}

	paths := make([]string, len(dirs))
	for i, dir := range dirs {
		paths[i] = filepath.Join(dir, "tabby", "tabby.db")
	}

	return paths
}

// legacyPath returns the first database in a location that an earlier version
// used when path does not exist, and "" otherwise.
func legacyPath(path string) string {
	if exists(path) {
		return ""
	}

	for _, legacy := range legacyPaths() {
		if legacy != path && exists(legacy) {
			return legacy
		}
	}

	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Open() (*sql.DB, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}

	// A database that another process has open cannot be moved. It is used
	// where it is, and the move is tried again the next time.
	if legacy := legacyPath(path); legacy != "" {
		if err := moveDatabase(legacy, path); err != nil {
			path = legacy
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := restrict(dir, 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path+pragmas)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	for _, file := range []string{path, path + "-wal", path + "-shm"} {
		if err := restrict(file, 0600); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return db, nil
}

// moveDatabase moves the database at from to to. It fails without moving
// anything while another process has the database open.
func moveDatabase(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", from+"?_pragma=busy_timeout(1000)")
	if err != nil {
		return err
	}

	// Leaving WAL mode checkpoints the log and deletes the -wal and -shm
	// files, so that only the main file has to be moved.
	var mode string
	err = db.QueryRow(`PRAGMA journal_mode = DELETE`).Scan(&mode)
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(mode, "delete") {
		return fmt.Errorf("database %s is in use", from)
	}

	if err := os.Rename(from, to); err != nil {
		// The locations can be on different file systems, which rename does
		// not support.
		if err := copyFile(from, to); err != nil {
			return err
		}
		if err := os.Remove(from); err != nil {
			_ = os.Remove(to)
			return err
		}
	}

	// Only succeeds when the old directory is empty.
	_ = os.Remove(filepath.Dir(from))

	return nil
}

// copyFile copies from to to through a temporary file, so that to is either
// complete or missing.
func copyFile(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()

	temp, err := os.CreateTemp(filepath.Dir(to), filepath.Base(to)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temp.Name()) }()

	_, err = io.Copy(temp, source)
	if err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	return os.Rename(temp.Name(), to)
}

// scrub rebuilds the database and empties the write-ahead log, so that deleted
// commands do not remain in free pages or in old log frames.
func scrub(db *sql.DB) error {
	if _, err := db.Exec(`VACUUM`); err != nil {
		return err
	}

	_, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// restrict tightens permissions on an existing path. Windows relies on the
// ACLs of the user profile instead.
func restrict(path string, mode fs.FileMode) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode().Perm()&^mode == 0 {
		return nil
	}

	return os.Chmod(path, mode)
}
