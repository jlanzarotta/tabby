package matcher

import (
	"sort"
	"time"
	"unicode"

	"tabby/internal/store"
)

type matchedCommand struct {
	RankedCommand
	textScore
	pick float64
}

// Context holds the extra scores that depend on where and how a query runs.
// Either map may be nil.
type Context struct {
	// Local holds scores from LocalScores, keyed by command text.
	Local map[string]float64
	// Picks holds scores from PickScores, keyed by command text.
	Picks map[string]float64
}

type textScore struct {
	tokenStarts int
	consecutive int
	gaps        int
}

type matchScratch struct {
	scores []textScore
}

// Query returns the commands that match query, best first. Commands picked
// for query come before all other matches.
func Query(commands []store.Command, query string, context Context) []RankedCommand {
	queryRunes := lowerRunes(nil, query)
	if len(queryRunes) <= 1 {
		return nil
	}

	now := time.Now()
	var commandRunes []rune
	var scratch matchScratch
	matches := make([]matchedCommand, 0, len(commands))
	for _, command := range commands {
		commandRunes = lowerRunes(commandRunes, command.Text)
		score, ok := match(commandRunes, queryRunes, &scratch)
		if ok {
			matches = append(matches, matchedCommand{
				RankedCommand: RankedCommand{
					Command: command,
					Score:   rankScore(command, now, context.Local[command.Text]),
				},
				textScore: score,
				pick:      context.Picks[command.Text],
			})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].pick != matches[j].pick {
			return matches[i].pick > matches[j].pick
		}
		if matches[i].textScore == matches[j].textScore {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].textScore.betterThan(matches[j].textScore)
	})

	result := make([]RankedCommand, len(matches))
	for i, matched := range matches {
		result[i] = matched.RankedCommand
	}

	return result
}

func match(command, query []rune, scratch *matchScratch) (textScore, bool) {
	if len(query) == 0 || len(command) < len(query) {
		return textScore{}, false
	}

	current, next := scratch.prepare(len(command))
	for position, character := range command {
		if character == query[0] && tokenStart(command, position) {
			current[position].tokenStarts = 1
		}
	}

	for _, character := range query[1:] {
		clear(next)
		var gapScore textScore
		gapPosition := 0
		gapFound := false

		for position, commandCharacter := range command {
			previous := position - 2
			if previous >= 0 && current[previous].tokenStarts > 0 &&
				(!gapFound || betterGapSource(
					current[previous],
					previous,
					gapScore,
					gapPosition,
				)) {
				gapScore = current[previous]
				gapPosition = previous
				gapFound = true
			}

			if commandCharacter != character {
				continue
			}

			var candidate textScore
			found := false
			if position > 0 && current[position-1].tokenStarts > 0 {
				candidate = current[position-1]
				candidate.consecutive++
				found = true
			}

			if gapFound {
				gapped := gapScore
				gapped.gaps += position - gapPosition - 1
				if !found || gapped.betterThan(candidate) {
					candidate = gapped
					found = true
				}
			}

			if !found {
				continue
			}
			if tokenStart(command, position) {
				candidate.tokenStarts++
			}
			next[position] = candidate
		}

		current, next = next, current
	}

	var best textScore
	found := false
	for _, score := range current {
		if score.tokenStarts > 0 && (!found || score.betterThan(best)) {
			best = score
			found = true
		}
	}

	return best, found
}

func (scratch *matchScratch) prepare(length int) ([]textScore, []textScore) {
	required := length * 2
	if cap(scratch.scores) < required {
		scratch.scores = make([]textScore, required)
	} else {
		scratch.scores = scratch.scores[:required]
		clear(scratch.scores)
	}

	return scratch.scores[:length:length], scratch.scores[length:]
}

func betterGapSource(score textScore, position int, other textScore, otherPosition int) bool {
	if score.tokenStarts != other.tokenStarts {
		return score.tokenStarts > other.tokenStarts
	}
	if score.consecutive != other.consecutive {
		return score.consecutive > other.consecutive
	}
	// A future gap adds the same target position to both scores.
	return score.gaps-position < other.gaps-otherPosition
}

func lowerRunes(runes []rune, text string) []rune {
	runes = runes[:0]
	for _, character := range text {
		runes = append(runes, unicode.ToLower(character))
	}
	return runes
}

func (score textScore) betterThan(other textScore) bool {
	if score.tokenStarts != other.tokenStarts {
		return score.tokenStarts > other.tokenStarts
	}
	if score.consecutive != other.consecutive {
		return score.consecutive > other.consecutive
	}
	return score.gaps < other.gaps
}

func tokenStart(command []rune, position int) bool {
	return position == 0 || unicode.IsSpace(command[position-1])
}
