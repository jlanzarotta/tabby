package cli

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"tabby/internal/store"
)

type AddCommand struct {
	Command  string
	Query    string
	Previous string
	FromEnv  bool
}

type RemoveCommand struct {
	Command string
}

type ListCommand struct {
	PinnedOnly bool
}

type NextCommand struct {
	Previous string
	FromEnv  bool
}

type StatsCommand struct {
	Top   int
	Shell string
}

type ForgetCommand struct {
	Query string
	All   bool
}

type PinCommand struct {
	Command string
}

type UnpinCommand struct {
	Command string
}

type PrepareCommand struct{}

type QueryCommand struct {
	Text    string
	FromEnv bool
}

type InitCommand struct {
	Shell string
}

type ExcludeAddCommand struct {
	Pattern string
	Force   bool
	DryRun  bool
}

type ExcludeTestCommand struct {
	Command string
}

type PruneCommand struct {
	OlderThan string
	DryRun    bool
}

type DoctorCommand struct {
	Version string
}

type ExcludeRemoveCommand struct {
	Pattern string
}

type ExcludeListCommand struct{}

type App struct {
	DB *sql.DB
}

const fromEnvHelp = "Read the value from the TABBY_INPUT environment variable"

var BuildVersion string
var BuildDateTime string

// Init parses the command line and runs the selected command.
func Init(version string) error {
	root := newRootCommand(version)

	// pflag treats "-version" as a cluster of shorthand flags.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "-version" {
		args = append([]string{"--version"}, args[1:]...)
	}
	root.SetArgs(args)

	return root.Execute()
}

const rootDescription = "Tabby is a fuzzy tab completer for your shell history. It learns the commands you run " +
	"successfully and brings them back with a few keystrokes and the Tab key."

func newRootCommand(version string) *cobra.Command {
	cobra.EnableCommandSorting = false

	root := &cobra.Command{
		Use:               "tabby",
		Short:             "Fuzzy tab autocompleter for successfully run commands",
		Long:              color.YellowString(TabbyTitle()) + "\n\n" + rootDescription,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion, _ := cmd.Flags().GetBool("version"); showVersion {
				runVersion(cmd, args)
				return nil
			}
			return cmd.Help()
		},
	}
	root.Flags().Bool("version", false, "Show the version information")

	root.AddCommand(
		newInitCommand(),
		newAddCommand(),
		newRemoveCommand(),
		newListCommand(),
		newPinCommand(),
		newUnpinCommand(),
		newForgetCommand(),
		newPruneCommand(),
		newStatsCommand(),
		newDoctorCommand(version),
		newPrepareCommand(),
		newQueryCommand(),
		newNextCommand(),
		newExcludeCommand(),
	)

	return root
}

// withDB opens the store for commands that need it and closes it afterward.
func withDB(run func(app *App, args []string) error) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, args []string) error {
		db, err := store.Open()
		if err != nil {
			return err
		}

		defer db.Close()

		return run(&App{DB: db}, args)
	}
}

func optionalArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:       "init <shell>",
		Short:     "Print shell integration",
		ValidArgs: []string{"zsh", "bash", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(_ *cobra.Command, args []string) error {
			cmd := InitCommand{Shell: args[0]}
			return cmd.Run()
		},
	}
}

func newAddCommand() *cobra.Command {
	var cmd AddCommand
	command := &cobra.Command{
		Use:   "add [command]",
		Short: "Add a new command to the store",
		Args:  cobra.MaximumNArgs(1),
		RunE: withDB(func(app *App, args []string) error {
			cmd.Command = optionalArg(args)
			return cmd.Run(app)
		}),
	}
	command.Flags().BoolVar(&cmd.FromEnv, "from-env", false, fromEnvHelp)
	_ = command.Flags().MarkHidden("from-env")
	command.Flags().StringVar(&cmd.Query, "query", "", "Record that the command was picked as a completion of this query")
	_ = command.Flags().MarkHidden("query")
	command.Flags().StringVar(&cmd.Previous, "previous", "", "Record that the command followed this command")
	_ = command.Flags().MarkHidden("previous")

	return command
}

func newRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <command>",
		Short: "Remove a command from the store",
		Args:  cobra.ExactArgs(1),
		RunE: withDB(func(app *App, args []string) error {
			cmd := RemoveCommand{Command: args[0]}
			return cmd.Run(app)
		}),
	}
}

func newListCommand() *cobra.Command {
	var cmd ListCommand
	command := &cobra.Command{
		Use:   "list",
		Short: "List all commands in the store",
		Args:  cobra.NoArgs,
		RunE: withDB(func(app *App, _ []string) error {
			return cmd.Run(app)
		}),
	}
	command.Flags().BoolVar(&cmd.PinnedOnly, "pinned", false, "List only pinned commands")

	return command
}

func newNextCommand() *cobra.Command {
	var cmd NextCommand
	command := &cobra.Command{
		Use:   "next [previous]",
		Short: "Predict the commands that usually follow a command",
		Args:  cobra.MaximumNArgs(1),
		RunE: withDB(func(app *App, args []string) error {
			cmd.Previous = optionalArg(args)
			return cmd.Run(app)
		}),
	}
	command.Flags().BoolVar(&cmd.FromEnv, "from-env", false, fromEnvHelp)
	_ = command.Flags().MarkHidden("from-env")

	return command
}

func newForgetCommand() *cobra.Command {
	var cmd ForgetCommand
	command := &cobra.Command{
		Use:   "forget [query]",
		Short: "Forget the commands learned for a query",
		Args:  cobra.MaximumNArgs(1),
		RunE: withDB(func(app *App, args []string) error {
			cmd.Query = optionalArg(args)
			return cmd.Run(app)
		}),
	}
	command.Flags().BoolVar(&cmd.All, "all", false, "Forget the learned commands for every query")

	return command
}

func newStatsCommand() *cobra.Command {
	var cmd StatsCommand
	command := &cobra.Command{
		Use:   "stats",
		Short: "Show usage statistics and alias suggestions",
		Args:  cobra.NoArgs,
		RunE: withDB(func(app *App, _ []string) error {
			return cmd.Run(app)
		}),
	}
	command.Flags().IntVar(&cmd.Top, "top", 10, "Number of most used commands to show")
	command.Flags().StringVar(&cmd.Shell, "shell", defaultShell(), "Shell syntax for alias suggestions: zsh, bash, or powershell")

	return command
}

func newPruneCommand() *cobra.Command {
	var cmd PruneCommand
	command := &cobra.Command{
		Use:   "prune",
		Short: "Delete commands that have not been used recently",
		Args:  cobra.NoArgs,
		RunE: withDB(func(app *App, _ []string) error {
			return cmd.Run(app)
		}),
	}
	command.Flags().StringVar(&cmd.OlderThan, "older-than", "", "Delete what was last used longer ago than this, such as 36h, 10d, or 2w")
	command.Flags().BoolVar(&cmd.DryRun, "dry-run", false, "Show what would be deleted without deleting it")
	_ = command.MarkFlagRequired("older-than")

	return command
}

func newDoctorCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the installation, the database, and the shell integration",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cmd := DoctorCommand{Version: version}
			return cmd.Run()
		},
	}
}

func newPinCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pin <command>",
		Short: "Pin a command so that it never expires",
		Args:  cobra.ExactArgs(1),
		RunE: withDB(func(app *App, args []string) error {
			cmd := PinCommand{Command: args[0]}
			return cmd.Run(app)
		}),
	}
}

func newUnpinCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unpin <command>",
		Short: "Unpin a command so that it expires normally",
		Args:  cobra.ExactArgs(1),
		RunE: withDB(func(app *App, args []string) error {
			cmd := UnpinCommand{Command: args[0]}
			return cmd.Run(app)
		}),
	}
}

func newPrepareCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "prepare",
		Short: "Prepare the database",
		Args:  cobra.NoArgs,
		RunE: withDB(func(_ *App, _ []string) error {
			var cmd PrepareCommand
			return cmd.Run()
		}),
	}
}

func newQueryCommand() *cobra.Command {
	var cmd QueryCommand
	command := &cobra.Command{
		Use:   "query [text]",
		Short: "Find matching commands",
		Args:  cobra.MaximumNArgs(1),
		RunE: withDB(func(app *App, args []string) error {
			cmd.Text = optionalArg(args)
			return cmd.Run(app)
		}),
	}
	command.Flags().BoolVar(&cmd.FromEnv, "from-env", false, fromEnvHelp)
	_ = command.Flags().MarkHidden("from-env")

	return command
}

func newExcludeCommand() *cobra.Command {
	exclude := &cobra.Command{
		Use:   "exclude",
		Short: "Manage exclusions",
		Args:  cobra.NoArgs,
	}

	var add ExcludeAddCommand
	addCommand := &cobra.Command{
		Use:   "add <pattern>",
		Short: "Add an exclusion pattern",
		Args:  cobra.ExactArgs(1),
		RunE: withDB(func(app *App, args []string) error {
			add.Pattern = args[0]
			return add.Run(app)
		}),
	}
	addCommand.Flags().BoolVar(&add.Force, "force", false, "Allow a pattern that matches every command")
	addCommand.Flags().BoolVar(&add.DryRun, "dry-run", false, "Show the stored commands that would be deleted without adding the pattern")

	exclude.AddCommand(
		addCommand,
		&cobra.Command{
			Use:   "remove <pattern>",
			Short: "Remove an exclusion pattern",
			Args:  cobra.ExactArgs(1),
			RunE: withDB(func(app *App, args []string) error {
				cmd := ExcludeRemoveCommand{Pattern: args[0]}
				return cmd.Run(app)
			}),
		},
		&cobra.Command{
			Use:   "test <command>",
			Short: "Show whether a command would be recorded, and why not",
			Args:  cobra.ExactArgs(1),
			RunE: withDB(func(app *App, args []string) error {
				cmd := ExcludeTestCommand{Command: args[0]}
				return cmd.Run(app)
			}),
		},
		&cobra.Command{
			Use:   "list",
			Short: "List exclusion patterns",
			Args:  cobra.NoArgs,
			RunE: withDB(func(app *App, _ []string) error {
				var cmd ExcludeListCommand
				return cmd.Run(app)
			}),
		},
	)

	return exclude
}

func runVersion(_ *cobra.Command, _ []string) {
	fmt.Printf("%s\n\nVersion: %s\nBorn on: %s\n"+
		"------------------------------------------------------------------------------\n"+
		"BSD 3-Clause License\n\n"+
		"Copyright (c) 2018-%s, Jeff Lanzarotta All rights reserved.\n"+
		"\n"+
		"Redistribution and use in source and binary forms, with or without\n"+
		"modification, are permitted provided that the following conditions are met:\n"+
		"\n"+
		"1. Redistributions of source code must retain the above copyright notice, this\n"+
		"   list of conditions and the following disclaimer.\n"+
		"\n"+
		"2. Redistributions in binary form must reproduce the above copyright notice,\n"+
		"   this list of conditions and the following disclaimer in the documentation\n"+
		"   and/or other materials provided with the distribution.\n"+
		"\n"+
		"3. Neither the name of the copyright holder nor the names of its\n"+
		"   contributors may be used to endorse or promote products derived from\n"+
		"   this software without specific prior written permission.\n"+
		"\n"+
		"THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS \"AS IS\"\n"+
		"AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE\n"+
		"IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE\n"+
		"DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE\n"+
		"FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL\n"+
		"DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR\n"+
		"SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER\n"+
		"CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,\n"+
		"OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE\n"+
		"OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.\n"+
		"------------------------------------------------------------------------------\n",
		color.YellowString(TabbyTitle()),
		BuildVersion,
		BuildDateTime,
		time.Now().Format("2006"))
}

func TabbyTitle() string {
	art := `
 ___________     _____     _     _
|           |   |_   _|_ _| |__ | |__  _   _
| |<-       |     | |/ _' | '_ \| '_ \| | | |
|   Tab     |     | | (_| | |_) | |_) | |_| |
|       ->| |     |_|\__,_|_.__/|_.__/ \__, |
|___________|                          |___/`

	return art
}
