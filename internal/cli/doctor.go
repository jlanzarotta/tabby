package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"tabby/internal/store"
)

const (
	shellEnvironmentVariable      = "TABBY_SHELL"
	predictionEnvironmentVariable = "TABBY_PREDICTION"
)

var initLines = map[string]string{
	"zsh":        `eval "$(tabby init zsh)"`,
	"bash":       `eval "$(tabby init bash)"`,
	"powershell": `Invoke-Expression (& { (tabby init powershell | Out-String) })`,
}

// Run prints the report, and returns an error when it finds a problem so
// that the exit status shows it.
func (cmd *DoctorCommand) Run() error {
	var problems, notes []string
	row := func(label, value string) {
		fmt.Printf("%-12s %s\n", label, value)
	}

	fmt.Printf("Tabby %s\n\n", cmd.Version)

	executable, err := os.Executable()
	if err != nil {
		row("Executable", "unknown")
	} else {
		row("Executable", executable)
	}

	if onPath, err := exec.LookPath("tabby"); err != nil {
		problems = append(problems, "tabby is not on PATH. The shell integration runs tabby from PATH, so it cannot work.")
	} else if target := scoopTarget(onPath); executable != "" && !sameFile(target, executable) {
		problems = append(problems, fmt.Sprintf(
			"tabby on PATH is %s, not this executable. The shell integration runs the one on PATH, which can be a different version.",
			target,
		))
	}

	report, err := store.Inspect()
	if err != nil {
		problems = append(problems, fmt.Sprintf("The database cannot be read: %v", err))
	}

	switch {
	case report.Path == "":
	case !report.Exists:
		row("Database", report.Path+" (not created yet)")
		notes = append(notes, "The database is created the first time a command is recorded.")
	default:
		row("Database", fmt.Sprintf("%s (%s, log %s)", report.Path, formatBytes(report.Size), formatBytes(report.LogSize)))
	}

	if report.Moving {
		notes = append(notes, fmt.Sprintf(
			"The database is in a location that an earlier version used. It is moved to %s the next time it is opened.",
			report.Target,
		))
	}

	if report.Exists && err == nil {
		switch {
		case report.Version == report.Latest:
			row("Schema", fmt.Sprintf("version %d (current)", report.Version))
		case report.Version < report.Latest:
			row("Schema", fmt.Sprintf("version %d (upgraded to %d on next use)", report.Version, report.Latest))
		default:
			row("Schema", fmt.Sprintf("version %d (newer than %d)", report.Version, report.Latest))
			problems = append(problems, "The database was created by a newer version of Tabby, which this version cannot open. Upgrade Tabby.")
		}

		row("Integrity", report.Integrity)
		if report.Integrity != "ok" {
			problems = append(problems, "SQLite reports that the database is damaged. Back up the file before you change anything.")
		}

		if report.Version == report.Latest {
			row("Commands", fmt.Sprintf("%d stored, %d pinned", report.Stats.Commands, report.Stats.Pinned))
			row("Learned", fmt.Sprintf("%d picks, %d follow-ups", report.Stats.Picks, report.Stats.Transitions))
			row("Exclusions", fmt.Sprintf("%d patterns", report.Exclusions))
		}
	}

	row("Retention", retention())

	if shell := os.Getenv(shellEnvironmentVariable); shell == "" {
		row("Shell", "integration not loaded")
		problems = append(problems, fmt.Sprintf(
			"The shell integration is not loaded in this shell. Add this line to your shell profile, then open a new shell: %s",
			initLines[guessShell()],
		))
	} else {
		row("Shell", shell)
		if shell == "powershell" {
			notes = append(notes, predictionNote(row)...)
		}
	}

	if len(notes) > 0 {
		fmt.Println()
		fmt.Println("Notes:")
		for _, note := range notes {
			fmt.Printf("  - %s\n", note)
		}
	}

	fmt.Println()
	if len(problems) == 0 {
		fmt.Println("No problems found")
		return nil
	}

	fmt.Println("Problems:")
	for _, problem := range problems {
		fmt.Printf("  - %s\n", problem)
	}

	return fmt.Errorf("found %d problem(s)", len(problems))
}

// predictionNote prints the PSReadLine prediction source that the
// PowerShell integration recorded at the last prompt. Its gray inline
// suggestions are easy to mistake for Tabby.
func predictionNote(row func(label, value string)) []string {
	source := os.Getenv(predictionEnvironmentVariable)
	switch source {
	case "":
		return nil
	case "None":
		row("Prediction", "off")
		return nil
	}

	row("Prediction", source+" (PSReadLine)")
	return []string{
		"The gray text that PowerShell shows after the cursor comes from PSReadLine prediction, not Tabby, and " +
			"Right arrow accepts it. To turn it off, run: Set-PSReadLineOption -PredictionSource None",
	}
}

func retention() string {
	months := store.RetentionMonths()
	unit := "months"
	if months == 1 {
		unit = "month"
	}

	return fmt.Sprintf("%d %s after last use; pinned commands do not expire", months, unit)
}

func guessShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	if filepath.Base(os.Getenv("SHELL")) == "bash" {
		return "bash"
	}

	return "zsh"
}

// scoopTarget returns the program that a Scoop shim at path runs, or path
// when it is not a Scoop shim.
func scoopTarget(path string) string {
	data, err := os.ReadFile(strings.TrimSuffix(path, filepath.Ext(path)) + ".shim")
	if err != nil {
		return path
	}

	for _, line := range strings.Split(string(data), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(name) == "path" {
			return strings.Trim(strings.TrimSpace(value), `"`)
		}
	}

	return path
}

func sameFile(a, b string) bool {
	first, err := os.Stat(a)
	if err != nil {
		return false
	}
	second, err := os.Stat(b)
	if err != nil {
		return false
	}

	return os.SameFile(first, second)
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	value, suffix := float64(size)/unit, "KB"
	for _, next := range []string{"MB", "GB"} {
		if value < unit {
			break
		}
		value, suffix = value/unit, next
	}

	return fmt.Sprintf("%.1f %s", value, suffix)
}
