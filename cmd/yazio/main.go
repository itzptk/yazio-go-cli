package main

import (
	"fmt"
	"os"

	"github.com/itzptk/yazio-go-cli/internal/cli"
)

var version = "dev"

func main() {
	cmd, err := cli.NewRootCommand(os.Stdout, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
