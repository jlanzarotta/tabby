package store

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	_ "modernc.org/sqlite"
)

// Path returns the database path. It uses $XDG_CONFIG_HOME/tabby when
// XDG_CONFIG_HOME is set to an absolute path, on every platform, and falls
// back to the platform default otherwise. Relative values are ignored, as the
// XDG Base Directory Specification requires.
func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "tabby", "tabby.db"), nil
}

func configDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return dir, nil
	}

	if runtime.GOOS == "windows" {
		return os.UserConfigDir()
	}

	// os.UserConfigDir fails on a relative XDG_CONFIG_HOME instead of ignoring it.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config"), nil
}

func Open() (*sql.DB, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := restrict(dir, 0700); err != nil {
		return nil, err
	}

	// Pragmas in the DSN apply to every connection the pool opens.
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
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
