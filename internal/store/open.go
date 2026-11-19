package store

import (
	"database/sql"
	"errors"
	"fmt"
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

// Path returns the database path. It uses $XDG_CONFIG_HOME/tabby when
// XDG_CONFIG_HOME is set to an absolute path, on every platform, and falls
// back to the platform default otherwise. Relative values are ignored, as the
// XDG Base Directory Specification requires. On Windows, it returns the old
// roaming location while a database there has not been moved yet.
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
	dir, err := baseDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "tabby", "tabby.db"), nil
}

func baseDir() (string, error) {
	if dir := xdgConfigHome(); dir != "" {
		return dir, nil
	}

	// On Windows, this is %LocalAppData%, which does not roam with the user
	// profile the way %AppData% does.
	if runtime.GOOS == "windows" {
		return os.UserCacheDir()
	}

	// os.UserConfigDir fails on a relative XDG_CONFIG_HOME instead of ignoring it.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config"), nil
}

func xdgConfigHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return dir
	}

	return ""
}

// legacyPath returns the database under %AppData% when it exists and path does
// not, and "" otherwise.
func legacyPath(path string) string {
	if runtime.GOOS != "windows" || xdgConfigHome() != "" {
		return ""
	}

	appData := os.Getenv("AppData")
	if appData == "" {
		return ""
	}

	legacy := filepath.Join(appData, "tabby", "tabby.db")
	if exists(path) || !exists(legacy) {
		return ""
	}

	return legacy
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
		return err
	}

	// Only succeeds when the old directory is empty.
	_ = os.Remove(filepath.Dir(from))

	return nil
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
