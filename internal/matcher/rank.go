package matcher

import (
	"sort"
	"time"

	"tabby/internal/store"
)

const (
	sameDirWeight  = 3.0
	sameRepoWeight = 1.0
	pinnedAgeFloor = 2.0
)

type RankedCommand struct {
	Command store.Command
	Score   float64
}

// LocalScores returns the extra frecency of each command from its usage in
// the current directory and repository, keyed by command text. A run in the
// current directory also counts toward the repository.
func LocalScores(usages []store.DirUsage, now time.Time) map[string]float64 {
	if len(usages) == 0 {
		return nil
	}

	scores := make(map[string]float64, len(usages))
	for _, usage := range usages {
		weight := sameRepoWeight
		if usage.SameDir {
			weight += sameDirWeight
		}
		scores[usage.Command] += weight * float64(usage.UseCount) * AgeMultiplier(usage.LastUsed, now)
	}

	return scores
}

func rankScore(command store.Command, now time.Time, local float64) float64 {
	age := AgeMultiplier(command.LastUsed, now)
	if command.Pinned {
		age = max(age, pinnedAgeFloor)
	}

	return float64(command.UseCount)*age + local
}

func Rank(commands []store.Command) []RankedCommand {
	now := time.Now()
	ranked := make([]RankedCommand, 0, len(commands))
	for _, command := range commands {
		ranked = append(ranked, RankedCommand{
			Command: command,
			Score:   rankScore(command, now, 0),
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}
