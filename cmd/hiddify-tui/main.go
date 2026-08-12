package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	if len(remaining) >= 2 && remaining[0] == "profile" {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		daemon, err := client.DialUnix(ctx, *socket)
		cancel()
		if err != nil {
			fmt.Fprintf(stderr, "profile: %v\n", err)
			return cli.ExitUnavailable
		}
		defer daemon.Close()
		ctx, cancel = context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		command := remaining[1]
		switch command {
		case "list":
			if len(remaining) != 2 {
				return profileUsage(stderr)
			}
			return cli.ProfileList(ctx, daemon, *jsonOutput, stdout, stderr)
		case "show":
			if len(remaining) != 3 {
				return profileUsage(stderr)
			}
			return cli.ProfileShow(ctx, daemon, remaining[2], *jsonOutput, stdout, stderr)
		case "add":
			profileFlags := flag.NewFlagSet("profile add", flag.ContinueOnError)
			profileFlags.SetOutput(stderr)
			name := profileFlags.String("name", "", "profile name")
			active := profileFlags.Bool("activate", false, "make profile active")
			if err := profileFlags.Parse(remaining[2:]); err != nil || profileFlags.NArg() != 1 {
				return profileUsage(stderr)
			}
			return cli.ProfileAddRemote(ctx, daemon, profileFlags.Arg(0), *name, *active, *jsonOutput, stdout, stderr)
		case "add-file", "add-stdin":
			profileFlags := flag.NewFlagSet("profile "+command, flag.ContinueOnError)
			profileFlags.SetOutput(stderr)
			name := profileFlags.String("name", "", "profile name")
			active := profileFlags.Bool("activate", false, "make profile active")
			if err := profileFlags.Parse(remaining[2:]); err != nil || (command == "add-file" && profileFlags.NArg() != 1) || (command == "add-stdin" && profileFlags.NArg() != 0) {
				return profileUsage(stderr)
			}
			var content io.Reader = os.Stdin
			if command == "add-file" {
				file, err := os.Open(profileFlags.Arg(0))
				if err != nil {
					fmt.Fprintf(stderr, "profile add-file: %v\n", err)
					return cli.ExitUsage
				}
				defer file.Close()
				content = file
				if *name == "" {
					*name = filepath.Base(profileFlags.Arg(0))
				}
			}
			return cli.ProfileAddLocal(ctx, daemon, content, *name, *active, *jsonOutput, stdout, stderr)
		case "rename":
			if len(remaining) != 4 {
				return profileUsage(stderr)
			}
			return cli.ProfileRename(ctx, daemon, remaining[2], remaining[3], *jsonOutput, stdout, stderr)
		case "activate", "refresh":
			if len(remaining) != 3 {
				return profileUsage(stderr)
			}
			operation := daemon.SetActiveProfile
			if command == "refresh" {
				operation = daemon.RefreshProfile
			}
			return cli.ProfileOperation(ctx, command, remaining[2], operation, stdout, stderr)
		case "delete":
			if len(remaining) != 4 || remaining[3] != "--yes" {
				fmt.Fprintln(stderr, "profile delete requires --yes")
				return cli.ExitUsage
			}
			return cli.ProfileOperation(ctx, command, remaining[2], daemon.DeleteProfile, stdout, stderr)
		default:
			return profileUsage(stderr)
		}
	}
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

func profileUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] profile list|show ID|add [--name NAME] [--activate] URL|add-file [--name NAME] [--activate] FILE|add-stdin [--name NAME] [--activate]|rename ID NAME|activate ID|refresh ID|delete ID --yes")
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
