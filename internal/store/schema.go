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

type Exclusion struct {
	Pattern   string
	CreatedAt int64
}
