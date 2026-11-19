package store

import (
	"strings"
	"testing"

	"tabby/internal/glob"
)

func TestAddTrimsTrailingNewline(t *testing.T) {
	db, _ := openTemp(t)

	for _, command := range []string{"git status\n", "git status\r\n", "  git status\t"} {
		if err := Add(db, command, Location{}, "", ""); err != nil {
			t.Fatal(err)
		}
	}

	commands, err := List(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].Text != "git status" || commands[0].UseCount != 3 {
		t.Fatalf("List() = %+v, want git status used 3 times", commands)
	}
}

func TestAddSkipsInnerControlCharacters(t *testing.T) {
	db, _ := openTemp(t)

	if err := Add(db, "echo one\necho two", Location{}, "", ""); err != nil {
		t.Fatal(err)
	}

	commands, err := List(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 0 {
		t.Fatalf("List() = %+v, want nothing", commands)
	}
}

func TestRemoveTrimsCommand(t *testing.T) {
	db, _ := openTemp(t)

	if err := Add(db, "git status", Location{}, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := Remove(db, "  git status "); err != nil {
		t.Fatal(err)
	}

	commands, err := List(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 0 {
		t.Fatalf("List() = %+v, want nothing", commands)
	}
}

func TestRemoveReportsMissingCommand(t *testing.T) {
	db, _ := openTemp(t)

	err := Remove(db, "git status")
	if err == nil || !strings.Contains(err.Error(), "not stored") {
		t.Fatalf("Remove() = %v, want a not stored error", err)
	}
}

// TestGlobMatchesSQLite checks that internal/glob, which blocks new commands,
// agrees with SQLite GLOB, which the migrations used to delete stored ones.
func TestGlobMatchesSQLite(t *testing.T) {
	db, _ := openTemp(t)

	exclusions, err := ListExclusions(db)
	if err != nil {
		t.Fatal(err)
	}

	patterns := []string{
		"*", "?", "a?c", "[a-c]x", "[^a-c]x", "[]]", "[]a]", "[^]]", "[a-]", "[-a]", "[abc", "a[", "[", "]",
		"[z-a]", "[a-c-e]", "*[^ ]*", "**", "*?*", "[*]", "[?]", "\\*", "*\u00e4*", "[\u00e4-\u00f6]",
	}
	for _, exclusion := range exclusions {
		patterns = append(patterns, strings.ToLower(exclusion.Pattern))
	}

	texts := []string{
		"", "a", "abc", "ac", "bx", "dx", "ax", "]", "-", "[", "*", "?", "\\*", "a[", "b",
		"cd /tmp", "echo cd /tmp", "mysql -u root -psecret db", "mysql -u root -p db",
		"git clone https://user:pw@host/repo", "git clone https://host/repo", "gh auth --token=abc",
		"export github_token=ghp_x", "sqlcmd -s server -p secret", "gr\u00e4tz", "\u00f6",
		"podman login -u me -p secret registry", "net use z: \\\\server\\share /user:me",
		"-----begin openssh private key-----", "vault kv put secret/x value=1",
	}

	stmt, err := db.Prepare(`SELECT ? GLOB ?`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	for _, pattern := range patterns {
		for _, text := range texts {
			var want bool
			if err := stmt.QueryRow(text, pattern).Scan(&want); err != nil {
				t.Fatal(err)
			}
			if got := glob.Match(pattern, text); got != want {
				t.Errorf("glob.Match(%q, %q) = %v, SQLite GLOB = %v", pattern, text, got, want)
			}
		}
	}
}
