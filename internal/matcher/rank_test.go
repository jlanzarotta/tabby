package matcher

import (
	"testing"
	"time"

	"tabby/internal/store"
)

func TestQueryPrefersCommandsUsedHere(t *testing.T) {
	now := time.Now()
	commands := []store.Command{
		{Text: "go tool pprof", UseCount: 20, LastUsed: now.Unix()},
		{Text: "go test ./...", UseCount: 5, LastUsed: now.Unix()},
	}

	if got := Query(commands, "gt", nil); got[0].Command.Text != "go tool pprof" {
		t.Fatalf("without local usage, first match = %q, want %q", got[0].Command.Text, "go tool pprof")
	}

	local := LocalScores([]store.DirUsage{
		{Command: "go test ./...", UseCount: 5, LastUsed: now.Unix(), SameDir: true},
	}, now)
	if got := Query(commands, "gt", local); got[0].Command.Text != "go test ./..." {
		t.Fatalf("with local usage, first match = %q, want %q", got[0].Command.Text, "go test ./...")
	}
}

func TestLocalScoresWeighsDirectoryAboveRepository(t *testing.T) {
	now := time.Now()
	scores := LocalScores([]store.DirUsage{
		{Command: "make here", UseCount: 1, LastUsed: now.Unix(), SameDir: true},
		{Command: "make there", UseCount: 1, LastUsed: now.Unix(), SameDir: false},
	}, now)

	if scores["make here"] <= scores["make there"] {
		t.Fatalf("directory score %g is not above repository score %g", scores["make here"], scores["make there"])
	}
}

func TestPinnedCommandsDoNotDecay(t *testing.T) {
	now := time.Now()
	old := now.Add(-20 * 24 * time.Hour).Unix()
	pinned := store.Command{Text: "release", UseCount: 1, LastUsed: old, Pinned: true}
	unpinned := store.Command{Text: "release", UseCount: 1, LastUsed: old}

	if got := rankScore(pinned, now, 0); got != pinnedAgeFloor {
		t.Fatalf("pinned score = %g, want %g", got, pinnedAgeFloor)
	}
	if rankScore(unpinned, now, 0) >= rankScore(pinned, now, 0) {
		t.Fatal("unpinned command scored at least as high as the pinned one")
	}
}
