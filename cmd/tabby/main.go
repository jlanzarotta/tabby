package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"tabby/internal/cli"
)

var version = "dev"

func main() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = strings.TrimPrefix(info.Main.Version, "v")
		}
	}

	err := cli.Init(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tabby:", err)
		os.Exit(1)
	}
}
