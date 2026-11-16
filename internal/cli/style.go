package cli

import "os"

const (
	dimStyle       = "\x1b[2m"
	highlightStyle = "\x1b[48;5;238m\x1b[38;5;255m"
	resetStyle     = "\x1b[0m"
)

type styler struct {
	color bool
}

// newStyler enables color only when stdout is a terminal and NO_COLOR is
// unset, as described at https://no-color.org.
func newStyler() styler {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return styler{}
	}

	info, err := os.Stdout.Stat()
	if err != nil {
		return styler{}
	}

	return styler{color: info.Mode()&os.ModeCharDevice != 0}
}

func (s styler) dim(text string) string {
	if !s.color {
		return text
	}
	return dimStyle + text + resetStyle
}

func (s styler) highlight(command string) string {
	if !s.color {
		return command
	}
	return highlightStyle + " " + command + " " + resetStyle
}
