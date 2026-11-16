package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLocationFindsRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	location := NewLocation(nested)
	if location.Dir != normalizePath(nested) {
		t.Fatalf("Dir = %q, want %q", location.Dir, nested)
	}
	if location.Root != normalizePath(root) {
		t.Fatalf("Root = %q, want %q", location.Root, root)
	}
}

func TestNewLocationIgnoresRelativePaths(t *testing.T) {
	if location := NewLocation("relative/dir"); location != (Location{}) {
		t.Fatalf("NewLocation() = %+v, want zero", location)
	}
}

func TestNewLocationIgnoresRepositoryAtHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.Mkdir(filepath.Join(home, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	if location := NewLocation(home); location.Root != "" {
		t.Fatalf("Root = %q, want empty", location.Root)
	}
}
