package store

import (
	"path/filepath"
	"runtime"
	"testing"
)

func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	return home
}

func TestPathUsesAbsoluteXDGConfigHome(t *testing.T) {
	setHome(t)
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(config, "tabby", "tabby.db"); path != want {
		t.Fatalf("Path() = %q, want %q", path, want)
	}
}

func TestPathIgnoresRelativeXDGConfigHome(t *testing.T) {
	home := setHome(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	want, err := Path()
	if err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{".config", "~/.config", "relative/dir"} {
		t.Setenv("XDG_CONFIG_HOME", value)
		path, err := Path()
		if err != nil {
			t.Fatalf("XDG_CONFIG_HOME=%q: %v", value, err)
		}
		if path != want {
			t.Fatalf("XDG_CONFIG_HOME=%q: Path() = %q, want %q", value, path, want)
		}
	}

	if runtime.GOOS == "linux" {
		if linux := filepath.Join(home, ".config", "tabby", "tabby.db"); want != linux {
			t.Fatalf("Path() = %q, want %q", want, linux)
		}
	}
}
