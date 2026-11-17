package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"time"

	"tabby/internal/alias"
	"tabby/internal/store"
)

const maxAliasSuggestions = 5

func defaultShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}

	return "zsh"
}

func (cmd *StatsCommand) Run(app *App) error {
	switch cmd.Shell {
	case "zsh", "bash", "powershell":
	default:
		return fmt.Errorf("unknown shell %q; use zsh, bash, or powershell", cmd.Shell)
	}
	if cmd.Top < 0 {
		return fmt.Errorf("--top must not be negative")
	}

	stats, err := store.ReadStats(app.DB)
	if err != nil {
		return err
	}

	commands, err := store.List(app.DB)
	if err != nil {
		return err
	}

	style := newStyler()
	fmt.Println(style.dim("Usage over the last month, plus pinned commands"))
	fmt.Println()
	fmt.Printf("Commands stored:    %d (%d pinned)\n", stats.Commands, stats.Pinned)
	fmt.Printf("Runs recorded:      %d\n", stats.Runs)
	fmt.Printf("Learned picks:      %d\n", stats.Picks)
	fmt.Printf("Learned follow-ups: %d\n", stats.Transitions)

	sort.SliceStable(commands, func(i, j int) bool {
		return commands[i].UseCount > commands[j].UseCount
	})

	if top := commands[:min(cmd.Top, len(commands))]; len(top) > 0 {
		fmt.Println()
		fmt.Println("Most used:")
		for _, command := range top {
			fmt.Printf(
				"  %5d  %s  %s\n",
				command.UseCount,
				style.dim(time.Unix(command.LastUsed, 0).Format("2006-01-02 15:04")),
				style.highlight(command.Text),
			)
		}
	}

	suggestions := alias.Suggest(commands, maxAliasSuggestions, onPath)
	if len(suggestions) > 0 {
		fmt.Println()
		fmt.Println("Alias suggestions:")
		for _, suggestion := range suggestions {
			fmt.Printf(
				"  %s %s\n    %s\n",
				style.highlight(suggestion.Command.Text),
				style.dim(fmt.Sprintf("(%d runs)", suggestion.Command.UseCount)),
				alias.Format(suggestion, cmd.Shell),
			)
		}
	}

	return nil
}

func onPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
