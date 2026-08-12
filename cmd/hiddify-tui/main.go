package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
	"github.com/1suo/hiddify-tui/internal/tui"
	"github.com/charmbracelet/x/term"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) == 1 && term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) {
		snapshot, err := client.Snapshot(context.Background(), unavailableControl{})
		if err := tui.Run(snapshot, err); err != nil {
			fmt.Fprintf(os.Stderr, "tui: %v\n", err)
			os.Exit(cli.ExitRejected)
		}
		return
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("hiddify-tui", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print JSON")
	showVersion := flags.Bool("version", false, "print version")
	if err := flags.Parse(args); err != nil {
		return cli.ExitUsage
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return cli.ExitOK
	}

	remaining := flags.Args()
	if len(remaining) == 1 && remaining[0] == "status" {
		return cli.Status(context.Background(), unavailableControl{}, *jsonOutput, stdout, stderr)
	}
	if len(remaining) == 0 {
		fmt.Fprintln(stderr, "usage: hiddify-tui [--json] status")
		return cli.ExitUsage
	}
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] status")
	return cli.ExitUsage
}

type unavailableControl struct{}

func (unavailableControl) GetSnapshot(context.Context) (control.Snapshot, error) {
	return control.Snapshot{}, client.ErrUnavailable
}
