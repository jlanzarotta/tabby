package matcher

import (
	"testing"
	"time"

	"tabby/internal/store"
)

func TestNextOrdersByTransitionThenCommandScore(t *testing.T) {
	now := time.Now().Unix()
	week := time.Now().Add(-3 * 24 * time.Hour).Unix()
	transitions := []store.Transition{
		{Command: store.Command{Text: "git push", UseCount: 90, LastUsed: now}, UseCount: 1, LastUsed: week},
		{Command: store.Command{Text: "git commit", UseCount: 5, LastUsed: now}, UseCount: 3, LastUsed: now},
		{Command: store.Command{Text: "git status", UseCount: 40, LastUsed: now}, UseCount: 3, LastUsed: now},
	}

	got := Next(transitions)
	want := []string{"git status", "git commit", "git push"}
	for i, text := range want {
		if got[i].Command.Text != text {
			t.Fatalf("Next()[%d] = %q, want %q", i, got[i].Command.Text, text)
		}
	}
}
