package shell

import _ "embed"

//go:embed tabby.zsh
var Zsh string

//go:embed tabby.bash
var Bash string

//go:embed tabby.ps1
var PowerShell string
