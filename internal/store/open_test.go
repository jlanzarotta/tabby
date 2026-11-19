package store

import (
	"bytes"
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pressly/goose/v3"
)

func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LocalAppData", filepath.Join(home, "AppData", "Local"))
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	return home
}

// openTemp opens a new database in its own XDG_DATA_HOME.
func openTemp(t *testing.T) (*sql.DB, string) {
	t.Helper()
	setHome(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)

	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db, filepath.Join(data, "tabby", "tabby.db")
}

// assertNotOnDisk fails when the database at path or its log contains text.
func assertNotOnDisk(t *testing.T, path, text string) {
	t.Helper()
	for _, file := range []string{path, path + "-wal"} {
		data, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(text)) {
			t.Fatalf("%s contains %q", file, text)
		}
	}
}

func TestPathUsesAbsoluteXDGDataHome(t *testing.T) {
	setHome(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(data, "tabby", "tabby.db"); path != want {
		t.Fatalf("Path() = %q, want %q", path, want)
	}
}

func TestPathIgnoresRelativeXDGDataHome(t *testing.T) {
	home := setHome(t)
	want, err := Path()
	if err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{".local/share", "~/.local/share", "relative/dir"} {
		t.Setenv("XDG_DATA_HOME", value)
		path, err := Path()
		if err != nil {
			t.Fatalf("XDG_DATA_HOME=%q: %v", value, err)
		}
		if path != want {
			t.Fatalf("XDG_DATA_HOME=%q: Path() = %q, want %q", value, path, want)
		}
	}

	if runtime.GOOS == "linux" {
		if linux := filepath.Join(home, ".local", "share", "tabby", "tabby.db"); want != linux {
			t.Fatalf("Path() = %q, want %q", want, linux)
		}
	}
}

// createLegacy creates a database at dir/tabby/tabby.db that holds command.
func createLegacy(t *testing.T, dir, command string) string {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", dir)
	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := Add(db, command, Location{}, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", "")

	return filepath.Join(dir, "tabby", "tabby.db")
}

// assertMoved opens the database and checks that it was moved from legacy to
// want and still holds command.
func assertMoved(t *testing.T, legacy, want, command string) {
	t.Helper()
	if path, err := Path(); err != nil || path != legacy {
		t.Fatalf("Path() before the move = %q, %v, want %q", path, err, legacy)
	}

	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if path, err := Path(); err != nil || path != want {
		t.Fatalf("Path() after the move = %q, %v, want %q", path, err, want)
	}
	if exists(legacy) {
		t.Fatalf("%s was not moved", legacy)
	}

	commands, err := List(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].Text != command {
		t.Fatalf("List() = %+v, want %q", commands, command)
	}
}

func TestOpenMovesDatabaseFromXDGConfigHome(t *testing.T) {
	setHome(t)
	config := t.TempDir()
	legacy := createLegacy(t, config, "echo from config")

	data := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_DATA_HOME", data)

	assertMoved(t, legacy, filepath.Join(data, "tabby", "tabby.db"), "echo from config")
}

func TestOpenMovesDatabaseFromDotConfig(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("~/.config was only the default on Linux")
	}

	home := setHome(t)
	legacy := createLegacy(t, filepath.Join(home, ".config"), "echo from dot config")

	assertMoved(t, legacy, filepath.Join(home, ".local", "share", "tabby", "tabby.db"), "echo from dot config")
}

func TestOpenMovesDatabaseFromRoamingProfile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the roaming profile only exists on Windows")
	}

	home := setHome(t)
	legacy := createLegacy(t, filepath.Join(home, "AppData", "Roaming"), "echo from roaming")

	assertMoved(t, legacy, filepath.Join(home, "AppData", "Local", "tabby", "tabby.db"), "echo from roaming")
}

func TestOpenKeepsExistingDatabaseOverLegacy(t *testing.T) {
	setHome(t)
	config := t.TempDir()
	legacy := createLegacy(t, config, "echo old")
	data := t.TempDir()
	current := createLegacy(t, data, "echo new")

	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_DATA_HOME", data)
	if path, err := Path(); err != nil || path != current {
		t.Fatalf("Path() = %q, %v, want %q", path, err, current)
	}
	if !exists(legacy) {
		t.Fatalf("%s was removed", legacy)
	}
}

func TestDeletedCommandsDoNotRemainOnDisk(t *testing.T) {
	db, path := openTemp(t)

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

	assertNotOnDisk(t, path, marker)
}

func TestDirectoriesAndQueriesAreNotStoredInPlainText(t *testing.T) {
	db, path := openTemp(t)

	root := filepath.Join(t.TempDir(), "acmeclientrepo")
	location := Location{Dir: filepath.Join(root, "src"), Root: root}
	if err := Add(db, "make build", location, "zqxjvquery", ""); err != nil {
		t.Fatal(err)
	}

	usages, err := ListLocal(db, location)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages) != 1 || !usages[0].SameDir {
		t.Fatalf("ListLocal() = %+v, want one usage in the same directory", usages)
	}

	other, err := ListLocal(db, Location{Dir: filepath.Join(root, "docs"), Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 || other[0].SameDir {
		t.Fatalf("ListLocal() elsewhere in the repository = %+v, want one usage in another directory", other)
	}

	picks, err := ListPicks(db, "ZQXJVQUERY ")
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 || picks[0].Command != "make build" {
		t.Fatalf("ListPicks() = %+v, want make build", picks)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	assertNotOnDisk(t, path, "acmeclientrepo")
	assertNotOnDisk(t, path, "zqxjvquery")
}

func TestMigrationHashesStoredValues(t *testing.T) {
	setHome(t)
	path := filepath.Join(t.TempDir(), "tabby.db")
	db, err := sql.Open("sqlite", path+pragmas)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(context.Background(), 7); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "acmeclientrepo")
	for _, insert := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO commands (command, use_count, last_used_at) VALUES ('make build', 1, unixepoch())`, nil},
		{`INSERT INTO command_dirs (command, dir, root, use_count, last_used_at) VALUES ('make build', ?, ?, 1, unixepoch())`,
			[]any{root, root}},
		{`INSERT INTO command_dirs (command, dir, root, use_count, last_used_at) VALUES ('make build', ?, '', 1, unixepoch())`,
			[]any{filepath.Join(t.TempDir(), "plain")}},
		{`INSERT INTO picks (query, command, use_count, last_used_at) VALUES ('mak', 'make build', 1, unixepoch())`, nil},
	} {
		if _, err := db.Exec(insert.query, insert.args...); err != nil {
			t.Fatal(err)
		}
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}

	usages, err := ListLocal(db, Location{Dir: root, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(usages) != 1 || !usages[0].SameDir {
		t.Fatalf("ListLocal() = %+v, want one usage in the same directory", usages)
	}

	var emptyRoots int
	if err := db.QueryRow(`SELECT count(*) FROM command_dirs WHERE root = ''`).Scan(&emptyRoots); err != nil {
		t.Fatal(err)
	}
	if emptyRoots != 1 {
		t.Fatalf("%d rows without a repository, want 1", emptyRoots)
	}

	picks, err := ListPicks(db, "mak")
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 {
		t.Fatalf("ListPicks() = %+v, want one pick", picks)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	assertNotOnDisk(t, path, "acmeclientrepo")
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

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from")
	to := filepath.Join(dir, "to")
	if err := os.WriteFile(from, []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(from, to); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(to)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "contents" {
		t.Fatalf("copied %q, want %q", data, "contents")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("%d files in %s, want 2", len(entries), dir)
	}
}
