package alias

import (
	"testing"

	"tabby/internal/store"
)

func TestSuggestOrdersBySavingsAndSkipsShortOrRareCommands(t *testing.T) {
	commands := []store.Command{
		{Text: "docker compose up -d --build", UseCount: 10},
		{Text: "podman compose -f deploy/compose.yml up -d", UseCount: 20},
		{Text: "go test ./...", UseCount: 100},
		{Text: "kubectl get pods --all-namespaces", UseCount: 2},
	}

	got := Suggest(commands, 5, func(string) bool { return false })
	if len(got) != 2 {
		t.Fatalf("Suggest() returned %d suggestions, want 2", len(got))
	}
	if got[0].Name != "pcfdud" || got[1].Name != "dcudb" {
		t.Fatalf("names = %q, %q, want %q, %q", got[0].Name, got[1].Name, "pcfdud", "dcudb")
	}
}

func TestSuggestAvoidsTakenNames(t *testing.T) {
	commands := []store.Command{
		{Text: "git status --short --branch", UseCount: 50},
		{Text: "git stash save --keep-index", UseCount: 40},
	}

	taken := func(name string) bool { return name == "gssb" }
	got := Suggest(commands, 5, taken)
	if got[0].Name != "gssb2" {
		t.Fatalf("first name = %q, want %q", got[0].Name, "gssb2")
	}
	if got[1].Name != "gssk" {
		t.Fatalf("second name = %q, want %q", got[1].Name, "gssk")
	}
}

func TestFormat(t *testing.T) {
	s := Suggestion{Command: store.Command{Text: "echo 'hi' there"}, Name: "eht"}

	if got, want := Format(s, "bash"), `alias eht='echo '\''hi'\'' there'`; got != want {
		t.Fatalf("Format(bash) = %s, want %s", got, want)
	}
	if got, want := Format(s, "powershell"), "function eht { echo 'hi' there }"; got != want {
		t.Fatalf("Format(powershell) = %s, want %s", got, want)
	}
}
