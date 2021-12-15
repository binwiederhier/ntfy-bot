// ntfybot is a bot that provides an interface to a ntfy server
package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"ntfy-bot/cmd"
	"os"
	"runtime"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
cli.AppHelpTemplate += fmt.Sprintf(`
Try 'ntfybot COMMAND --help' for more information.

ntfybot %s (%s), runtime %s, built at %s
Copyright (C) 2021 Philipp C. Heckel, distributed under the Apache License 2.0
`, version, commit[:7], runtime.Version(), date)

	app := cmd.New()
	app.Version = version

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
