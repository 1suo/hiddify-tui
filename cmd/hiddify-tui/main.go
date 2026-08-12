package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
	"github.com/1suo/hiddify-tui/internal/tui"
	"github.com/charmbracelet/x/term"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) == 1 && term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) {
		dialCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		daemon, err := client.DialUnix(dialCtx, client.DefaultSocket())
		cancel()
		if err == nil {
			defer daemon.Close()
		}
		if err == nil {
			if err := tui.RunLive(context.Background(), daemon, daemon); err != nil {
				fmt.Fprintf(os.Stderr, "tui: %v\n", err)
				os.Exit(cli.ExitRejected)
			}
			return
		}
		if err := tui.Run(control.Snapshot{}, err); err != nil {
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
	socket := flags.String("socket", client.DefaultSocket(), "local control socket")
	timeout := flags.Duration("timeout", 3*time.Second, "daemon request timeout")
	showVersion := flags.Bool("version", false, "print version")
	if err := flags.Parse(args); err != nil {
		return cli.ExitUsage
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return cli.ExitOK
	}

	remaining := flags.Args()
	if len(remaining) >= 1 && remaining[0] == "status" {
		watch := len(remaining) == 2 && remaining[1] == "--watch"
		if len(remaining) > 1 && !watch {
			fmt.Fprintln(stderr, "usage: hiddify-tui [--json] [--socket PATH] [--timeout DURATION] status [--watch]")
			return cli.ExitUsage
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		daemon, err := client.DialUnix(ctx, *socket)
		cancel()
		if err != nil {
			return cli.Status(context.Background(), unavailableControl{err: err}, *jsonOutput, stdout, stderr)
		}
		defer daemon.Close()
		if watch {
			return cli.StatusWatch(context.Background(), daemon, daemon, *jsonOutput, stdout, stderr)
		}
		ctx, cancel = context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		return cli.Status(ctx, daemon, *jsonOutput, stdout, stderr)
	}
	if len(remaining) == 0 {
		fmt.Fprintln(stderr, "usage: hiddify-tui [--json] status")
		return cli.ExitUsage
	}
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] status")
	return cli.ExitUsage
}

type unavailableControl struct {
	err error
}

func (u unavailableControl) GetSnapshot(context.Context) (control.Snapshot, error) {
	if u.err != nil {
		return control.Snapshot{}, u.err
	}
	return control.Snapshot{}, client.ErrUnavailable
}
