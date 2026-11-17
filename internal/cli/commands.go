package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"tabby/internal/matcher"
	"tabby/internal/shell"
	"tabby/internal/store"
)

const (
	inputEnvironmentVariable    = "TABBY_INPUT"
	queryEnvironmentVariable    = "TABBY_QUERY"
	previousEnvironmentVariable = "TABBY_PREVIOUS"
)

// input returns the positional value, or the value of TABBY_INPUT when
// fromEnv is set. PowerShell 5.1 passes the text through the environment
// because it does not escape embedded quotes in native command arguments.
func input(value string, fromEnv bool) (string, error) {
	if !fromEnv {
		return value, nil
	}
	if value != "" {
		return "", fmt.Errorf("--from-env cannot be combined with a positional argument")
	}

	return os.Getenv(inputEnvironmentVariable), nil
}

func (cmd *AddCommand) Run(app *App) error {
	command, err := input(cmd.Command, cmd.FromEnv)
	if err != nil {
		return err
	}

	query := cmd.Query
	if cmd.FromEnv && query == "" {
		query = os.Getenv(queryEnvironmentVariable)
	}

	previous := cmd.Previous
	if cmd.FromEnv && previous == "" {
		previous = os.Getenv(previousEnvironmentVariable)
	}

	return store.Add(app.DB, command, store.CurrentLocation(), query, previous)
}

func (cmd *RemoveCommand) Run(app *App) error {
	return store.Remove(app.DB, cmd.Command)
}

func (cmd *ListCommand) Run(app *App) error {
	commands, err := store.List(app.DB)
	if err != nil {
		return err
	}

	style := newStyler()
	for _, ranked := range matcher.Rank(commands) {
		command := ranked.Command
		if cmd.PinnedOnly && !command.Pinned {
			continue
		}

		pin := ""
		if command.Pinned {
			pin = " " + style.dim("[pinned]")
		}

		fmt.Printf(
			"%s %s (Score: %g)%s\n",
			style.dim(time.Unix(command.LastUsed, 0).Format("2006-01-02 15:04")),
			style.highlight(command.Text),
			ranked.Score,
			pin,
		)
	}

	return nil
}

func (cmd *PinCommand) Run(app *App) error {
	return store.Pin(app.DB, cmd.Command)
}

func (cmd *UnpinCommand) Run(app *App) error {
	return store.Unpin(app.DB, cmd.Command)
}

func (cmd *NextCommand) Run(app *App) error {
	previous, err := input(cmd.Previous, cmd.FromEnv)
	if err != nil {
		return err
	}

	transitions, err := store.ListNext(app.DB, previous)
	if err != nil {
		return err
	}

	results := matcher.Next(transitions)
	results = results[:min(len(results), 50)]

	for _, ranked := range results {
		fmt.Println(ranked.Command.Text)
	}

	return nil
}

func (cmd *ForgetCommand) Run(app *App) error {
	if cmd.Query == "" && !cmd.All {
		return fmt.Errorf("pass a query, or --all to forget every learned pick")
	}
	if cmd.Query != "" && cmd.All {
		return fmt.Errorf("--all cannot be combined with a query")
	}

	forgotten, err := store.ForgetPicks(app.DB, cmd.Query)
	if err != nil {
		return err
	}

	fmt.Printf("Forgot %d learned pick(s)\n", forgotten)
	return nil
}

func (cmd *PathCommand) Run() error {
	path, err := store.Path()
	if err != nil {
		return err
	}

	fmt.Println(path)
	return nil
}

func (cmd *PrepareCommand) Run() error {
	return nil
}

func (cmd *QueryCommand) Run(app *App) error {
	text, err := input(cmd.Text, cmd.FromEnv)
	if err != nil {
		return err
	}

	if strings.IndexFunc(text, unicode.IsSpace) >= 0 {
		return fmt.Errorf("query must not contain spaces")
	}
	if looksLikePath(text) {
		return fmt.Errorf("query must not be a path")
	}

	commands, err := store.List(app.DB)
	if err != nil {
		return err
	}

	usages, err := store.ListLocal(app.DB, store.CurrentLocation())
	if err != nil {
		return err
	}

	picks, err := store.ListPicks(app.DB, text)
	if err != nil {
		return err
	}

	now := time.Now()
	results := matcher.Query(commands, text, matcher.Context{
		Local: matcher.LocalScores(usages, now),
		Picks: matcher.PickScores(picks, now),
	})
	results = results[:min(len(results), 50)]

	for _, ranked := range results {
		fmt.Println(ranked.Command.Text)
	}

	return nil
}

func looksLikePath(text string) bool {
	if text == "." || text == ".." || strings.HasPrefix(text, "~") || strings.ContainsAny(text, `/\`) {
		return true
	}

	return len(text) >= 2 && text[1] == ':' &&
		(text[0] >= 'a' && text[0] <= 'z' || text[0] >= 'A' && text[0] <= 'Z')
}

func (cmd *ExcludeAddCommand) Run(app *App) error {
	if strings.Trim(cmd.Pattern, "*") == "" && !cmd.Force {
		return fmt.Errorf("pattern %q matches every command and would delete all history; pass --force to add it anyway", cmd.Pattern)
	}

	removed, err := store.AddExclusion(app.DB, cmd.Pattern)
	if err != nil {
		return err
	}

	fmt.Printf("Removed %d matching stored command(s)\n", removed)
	return nil
}

func (cmd *ExcludeRemoveCommand) Run(app *App) error {
	return store.RemoveExclusion(app.DB, cmd.Pattern)
}

func (cmd *ExcludeListCommand) Run(app *App) error {
	exclusions, err := store.ListExclusions(app.DB)
	if err != nil {
		return err
	}

	style := newStyler()
	for _, exclusion := range exclusions {
		fmt.Printf(
			"%s %s\n",
			style.dim(time.Unix(exclusion.CreatedAt, 0).Format("2006-01-02 15:04")),
			style.highlight(exclusion.Pattern),
		)
	}

	return nil
}

func (cmd *InitCommand) Run() error {
	switch cmd.Shell {
	case "zsh":
		fmt.Print(shell.Zsh)
	case "bash":
		fmt.Print(shell.Bash)
	case "powershell":
		fmt.Print(shell.PowerShell)
	}
	return nil
}
