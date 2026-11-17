package alias

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"tabby/internal/store"
)

const (
	minUses   = 5
	minLength = 15
	maxNames  = 9
)

// Suggestion is a frequently run long command and a short name for it.
type Suggestion struct {
	Command store.Command
	Name    string
}

// Suggest returns up to limit commands worth an alias, the ones that would
// save the most typing first. taken reports whether a name is already in
// use, for example by a program on the PATH.
func Suggest(commands []store.Command, limit int, taken func(string) bool) []Suggestion {
	candidates := make([]store.Command, 0, len(commands))
	for _, command := range commands {
		if command.UseCount >= minUses && utf8.RuneCountInString(command.Text) >= minLength {
			candidates = append(candidates, command)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return savings(candidates[i]) > savings(candidates[j])
	})

	used := make(map[string]bool)
	var suggestions []Suggestion
	for _, command := range candidates {
		if len(suggestions) == limit {
			break
		}

		name := uniqueName(initials(command.Text), used, taken)
		if name == "" {
			continue
		}

		used[name] = true
		suggestions = append(suggestions, Suggestion{Command: command, Name: name})
	}

	return suggestions
}

// Format returns the definition of s for shell, which is "zsh", "bash", or
// "powershell".
func Format(s Suggestion, shell string) string {
	if shell == "powershell" {
		return "function " + s.Name + " { " + s.Command.Text + " }"
	}

	return "alias " + s.Name + "='" + strings.ReplaceAll(s.Command.Text, "'", `'\''`) + "'"
}

func savings(command store.Command) int {
	return command.UseCount * utf8.RuneCountInString(command.Text)
}

// initials returns the first letter or digit of each word of text, skipping
// leading punctuation such as the dashes of an option.
func initials(text string) string {
	var name strings.Builder
	for _, word := range strings.Fields(text) {
		for _, character := range word {
			if character < unicode.MaxASCII && (unicode.IsLetter(character) || unicode.IsDigit(character)) {
				name.WriteRune(unicode.ToLower(character))
				break
			}
		}
	}

	return name.String()
}

func uniqueName(base string, used map[string]bool, taken func(string) bool) string {
	if len(base) < 2 {
		return ""
	}

	for i := 1; i <= maxNames; i++ {
		name := base
		if i > 1 {
			name += strconv.Itoa(i)
		}
		if !used[name] && !taken(name) {
			return name
		}
	}

	return ""
}
