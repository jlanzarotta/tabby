package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const dirEnvironmentVariable = "TABBY_DIR"

// Location is where a command runs: the working directory and the root of
// the enclosing git repository, if there is one.
type Location struct {
	Dir  string
	Root string
}

// CurrentLocation returns the location named by TABBY_DIR, or the process
// working directory when TABBY_DIR is not set.
func CurrentLocation() Location {
	dir := os.Getenv(dirEnvironmentVariable)
	if dir == "" {
		dir, _ = os.Getwd()
	}

	return NewLocation(dir)
}

// NewLocation returns the location of dir. It returns a zero Location when
// dir is not an absolute path.
func NewLocation(dir string) Location {
	if dir == "" || !filepath.IsAbs(dir) {
		return Location{}
	}

	dir = filepath.Clean(dir)
	return Location{
		Dir:  normalizePath(dir),
		Root: normalizePath(repoRoot(dir)),
	}
}

// repoRoot returns the closest directory at or above dir that contains a
// .git entry. A repository at the home directory is ignored, because a
// dotfiles repository there would put every directory in the same project.
func repoRoot(dir string) string {
	home, _ := os.UserHomeDir()
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			if home != "" && normalizePath(dir) == normalizePath(filepath.Clean(home)) {
				return ""
			}
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func normalizePath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}

	return path
}
