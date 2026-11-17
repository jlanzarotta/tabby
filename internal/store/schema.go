package store

type Command struct {
	Text     string
	UseCount int
	LastUsed int64
	Pinned   bool
}

// DirUsage is the usage of a command in one directory.
type DirUsage struct {
	Command  string
	UseCount int
	LastUsed int64
	// SameDir is true when the usage was in the current directory, and false
	// when it was elsewhere in the current repository.
	SameDir bool
}

// Pick records that query was completed to command and the result was run.
type Pick struct {
	Command  string
	UseCount int
	LastUsed int64
}

// Transition records that command was run right after another command.
type Transition struct {
	// Command is the command that followed, with its own usage.
	Command  Command
	UseCount int
	LastUsed int64
}

// Stats summarizes the store.
type Stats struct {
	Commands    int
	Pinned      int
	Runs        int
	Picks       int
	Transitions int
}

type Exclusion struct {
	Pattern   string
	CreatedAt int64
}
