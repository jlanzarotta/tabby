package store

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestCheck(t *testing.T) {
	db, _ := openTemp(t)

	tests := []struct {
		command    string
		reason     string
		exclusions []string
	}{
		{"git status", "", nil},
		{"  git status  ", "", nil},
		{"ls -l", "it is five characters or fewer", nil},
		{"tabby list", "it runs tabby itself", nil},
		{"echo a\necho b", "it contains control characters", nil},
		{"cd /tmp", "it matches an exclusion pattern", []string{"cd *"}},
		{"gh auth login --token=abc", "it matches an exclusion pattern", []string{"*--token*", "*token=*", "*-token*"}},
	}

	for _, test := range tests {
		verdict, err := Check(db, test.command)
		if err != nil {
			t.Fatal(err)
		}
		if verdict.Reason != test.reason {
			t.Errorf("Check(%q).Reason = %q, want %q", test.command, verdict.Reason, test.reason)
		}

		got := append([]string(nil), verdict.Exclusions...)
		want := append([]string(nil), test.exclusions...)
		sort.Strings(got)
		sort.Strings(want)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("Check(%q).Exclusions = %q, want %q", test.command, got, want)
		}
	}
}

func TestMatchingCommandsDeletesNothing(t *testing.T) {
	db, _ := openTemp(t)

	for _, command := range []string{"git status", "git push", "go test ./..."} {
		if err := Add(db, command, Location{}, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := Pin(db, "git push"); err != nil {
		t.Fatal(err)
	}

	matched, err := MatchingCommands(db, "GIT *")
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 2 || matched[0].Text != "git push" || !matched[0].Pinned || matched[1].Text != "git status" {
		t.Fatalf("MatchingCommands() = %+v, want git push (pinned) and git status", matched)
	}

	commands, err := List(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 3 {
		t.Fatalf("List() = %+v, want all 3 commands kept", commands)
	}

	exclusions, err := ListExclusions(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, exclusion := range exclusions {
		if exclusion.Pattern == "GIT *" {
			t.Fatal("MatchingCommands() added the pattern")
		}
	}
}

func TestPrune(t *testing.T) {
	db, path := openTemp(t)

	location := NewLocation(t.TempDir())
	for _, command := range []string{"podman build .", "podman push image", "git status", "git pinned-command"} {
		if err := Add(db, command, location, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := Add(db, "podman push image", location, "pp", "podman build ."); err != nil {
		t.Fatal(err)
	}
	if err := Pin(db, "git pinned-command"); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-10 * 24 * time.Hour).Unix()
	for _, table := range []string{"commands", "command_dirs", "picks", "transitions"} {
		if _, err := db.Exec("UPDATE "+table+" SET last_used_at = ? WHERE command <> 'git status'", old); err != nil {
			t.Fatal(err)
		}
	}

	before := time.Now().Add(-7 * 24 * time.Hour)

	// podman build . and podman push image are old. Their directory usage
	// (2), the pick (1), and the follow-up (1) go with them. The pinned
	// command stays, but its old directory usage (1) is deleted.
	dry, err := Prune(db, before, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(dry.Commands) != 2 || dry.Records != 5 {
		t.Fatalf("Prune(dry run) = %d commands, %d records, want 2 and 5", len(dry.Commands), dry.Records)
	}

	commands, err := List(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 4 {
		t.Fatalf("dry run deleted commands: %+v", commands)
	}

	result, err := Prune(db, before, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Commands) != 2 || result.Records != 5 {
		t.Fatalf("Prune() = %d commands, %d records, want 2 and 5", len(result.Commands), result.Records)
	}

	commands, err = List(db)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, command := range commands {
		kept = append(kept, command.Text)
	}
	sort.Strings(kept)
	if strings.Join(kept, "|") != "git pinned-command|git status" {
		t.Fatalf("List() after Prune() = %q, want git pinned-command and git status", kept)
	}

	for _, table := range []string{"command_dirs", "picks", "transitions"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM " + table + " WHERE last_used_at <= " + strconv.FormatInt(before.Unix(), 10)).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%s has %d old rows after Prune()", table, count)
		}
	}

	assertNotOnDisk(t, path, "podman push image")

	again, err := Prune(db, before, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Commands) != 0 || again.Records != 0 {
		t.Fatalf("second Prune() = %+v, want nothing", again)
	}
}

func TestInspect(t *testing.T) {
	db, path := openTemp(t)

	if err := Add(db, "git status", Location{}, "", ""); err != nil {
		t.Fatal(err)
	}

	report, err := Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if report.Path != path || !report.Exists || report.Moving {
		t.Fatalf("Inspect() = %+v, want existing database at %s", report, path)
	}
	if report.Version != 8 || report.Latest != 8 || report.Integrity != "ok" {
		t.Fatalf("Inspect() = version %d of %d, integrity %q", report.Version, report.Latest, report.Integrity)
	}
	if report.Stats.Commands != 1 || report.Exclusions == 0 || report.Size == 0 {
		t.Fatalf("Inspect() = %+v, want 1 command, exclusions, and a size", report)
	}
}

func TestInspectDoesNotCreateDatabase(t *testing.T) {
	setHome(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)

	report, err := Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if report.Exists {
		t.Fatalf("Inspect() = %+v, want no database", report)
	}

	entries, err := os.ReadDir(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("Inspect() created %v", entries)
	}
}
