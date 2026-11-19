package store

import (
	"bytes"
	"database/sql"
	"os"
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
	t.Setenv("LocalAppData", filepath.Join(home, "AppData", "Local"))
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

func TestDeletedCommandsDoNotRemainOnDisk(t *testing.T) {
	setHome(t)
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)

	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}

	const marker = "zqxjvmarker"
	for _, command := range []string{"echo " + marker + " one", "echo " + marker + " two"} {
		if err := Add(db, command, Location{}, marker, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := Add(db, "echo kept", Location{}, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := Remove(db, "echo "+marker+" one"); err != nil {
		t.Fatal(err)
	}
	if _, err := AddExclusion(db, "echo zqx*"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(config, "tabby", "tabby.db")
	for _, file := range []string{path, path + "-wal"} {
		data, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(marker)) {
			t.Fatalf("%s still contains a deleted command", file)
		}
	}
}

func TestMoveDatabase(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "Roaming", "tabby", "tabby.db")
	to := filepath.Join(dir, "Local", "tabby", "tabby.db")
	if err := os.MkdirAll(filepath.Dir(from), 0700); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", from+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE t (v TEXT); INSERT INTO t VALUES ('moved')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := moveDatabase(from, to); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Dir(from)) {
		t.Fatalf("%s was not removed", filepath.Dir(from))
	}

	db, err = sql.Open("sqlite", to)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var value string
	if err := db.QueryRow(`SELECT v FROM t`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "moved" {
		t.Fatalf("value = %q, want %q", value, "moved")
	}
}

func TestMoveDatabaseFailsWhileInUse(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "Roaming", "tabby", "tabby.db")
	to := filepath.Join(dir, "Local", "tabby", "tabby.db")
	if err := os.MkdirAll(filepath.Dir(from), 0700); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", from+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatal(err)
	}

	if err := moveDatabase(from, to); err == nil {
		t.Fatal("moveDatabase() succeeded while the database was in use")
	}
	if !exists(from) || exists(to) {
		t.Fatal("moveDatabase() moved a database that was in use")
	}
}

func TestPathMovesOutOfRoamingProfile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the roaming profile only exists on Windows")
	}

	home := setHome(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	legacy := filepath.Join(home, "AppData", "Roaming", "tabby", "tabby.db")
	want := filepath.Join(home, "AppData", "Local", "tabby", "tabby.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, nil, 0600); err != nil {
		t.Fatal(err)
	}

	if path, err := Path(); err != nil || path != legacy {
		t.Fatalf("Path() = %q, %v, want %q", path, err, legacy)
	}

	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if path, err := Path(); err != nil || path != want {
		t.Fatalf("Path() = %q, %v, want %q", path, err, want)
	}
	if exists(legacy) {
		t.Fatalf("%s was not moved", legacy)
	}
}
