package matcher

import (
	"sort"
	"time"

	"tabby/internal/store"
)

// Next orders the commands that have followed a command, most likely first.
// Transitions are ranked by frecency, and ties by the score of the command.
func Next(transitions []store.Transition) []RankedCommand {
	now := time.Now()
	type candidate struct {
		RankedCommand
		transition float64
	}

	candidates := make([]candidate, 0, len(transitions))
	for _, transition := range transitions {
		candidates = append(candidates, candidate{
			RankedCommand: RankedCommand{
				Command: transition.Command,
				Score:   rankScore(transition.Command, now, 0),
			},
			transition: float64(transition.UseCount) * AgeMultiplier(transition.LastUsed, now),
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].transition != candidates[j].transition {
			return candidates[i].transition > candidates[j].transition
		}
		return candidates[i].Score > candidates[j].Score
	})

	result := make([]RankedCommand, len(candidates))
	for i, candidate := range candidates {
		result[i] = candidate.RankedCommand
	}

	return result
}
