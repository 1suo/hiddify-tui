package main

import (
	"context"
	"encoding/json"
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
	if len(remaining) >= 1 && (remaining[0] == "autoconnect" || remaining[0] == "service" || remaining[0] == "agent" || remaining[0] == "diagnostics") {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		daemon, err := client.DialUnix(ctx, *socket)
		cancel()
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", remaining[0], err)
			return cli.ExitUnavailable
		}
		defer daemon.Close()
		ctx, cancel = context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		switch {
		case len(remaining) == 2 && remaining[0] == "autoconnect" && (remaining[1] == "status" || remaining[1] == "enable" || remaining[1] == "disable"):
			return cli.AutoConnect(ctx, daemon, remaining[1], *jsonOutput, stdout, stderr)
		case len(remaining) == 2 && remaining[0] == "service" && remaining[1] == "status":
			return cli.ServiceStatus(ctx, daemon, *jsonOutput, stdout, stderr)
		case len(remaining) == 2 && remaining[0] == "agent" && remaining[1] == "status":
			return cli.AgentStatus(ctx, daemon, *jsonOutput, stdout, stderr)
		case len(remaining) == 1 && remaining[0] == "diagnostics":
			return cli.Diagnostics(ctx, daemon, *jsonOutput, stdout, stderr)
		default:
			return serviceUsage(stderr)
		}
	}
	if len(remaining) >= 2 && remaining[0] == "settings" {
		command := remaining[1]
		includeSecrets := false
		var candidate []byte
		switch command {
		case "show":
			if len(remaining) != 2 {
				return settingsUsage(stderr)
			}
		case "validate", "set", "import":
			if len(remaining) != 3 {
				return settingsUsage(stderr)
			}
			data, err := os.ReadFile(remaining[2])
			if err != nil || !json.Valid(data) {
				if err != nil {
					fmt.Fprintf(stderr, "settings %s: %v\n", command, err)
				} else {
					fmt.Fprintf(stderr, "settings %s: input is not valid JSON\n", command)
				}
				return cli.ExitUsage
			}
			candidate = data
		case "reset":
			if len(remaining) != 3 || remaining[2] != "--yes" {
				fmt.Fprintln(stderr, "settings reset requires --yes")
				return cli.ExitUsage
			}
		case "export":
			if len(remaining) == 2 {
				break
			}
			if len(remaining) == 4 && remaining[2] == "--include-secrets" && remaining[3] == "--yes" {
				includeSecrets = true
				break
			}
			return settingsUsage(stderr)
		default:
			return settingsUsage(stderr)
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		daemon, err := client.DialUnix(ctx, *socket)
		cancel()
		if err != nil {
			fmt.Fprintf(stderr, "settings: %v\n", err)
			return cli.ExitUnavailable
		}
		defer daemon.Close()
		ctx, cancel = context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		switch command {
		case "show":
			return cli.SettingsShow(ctx, daemon, stdout, stderr)
		case "validate":
			return cli.SettingsValidate(ctx, daemon, candidate, stdout, stderr)
		case "set", "import", "reset":
			return cli.SettingsWrite(ctx, daemon, command, candidate, stdout, stderr)
		default:
			return cli.SettingsExport(ctx, daemon, includeSecrets, stdout, stderr)
		}
	}
	if len(remaining) >= 1 && remaining[0] == "logs" {
		logFlags := flag.NewFlagSet("logs", flag.ContinueOnError)
		logFlags.SetOutput(stderr)
		follow := logFlags.Bool("follow", false, "follow new entries")
		tail := logFlags.Uint("tail", 100, "initial number of entries")
		level := logFlags.String("level", "info", "debug, info, warn, or error")
		if len(remaining) > 1 && remaining[1] == "clear" {
			if len(remaining) != 3 || remaining[2] != "--yes" {
				fmt.Fprintln(stderr, "logs clear requires --yes")
				return cli.ExitUsage
			}
		} else if err := logFlags.Parse(remaining[1:]); err != nil || logFlags.NArg() != 0 {
			return logsUsage(stderr)
		}
		logLevel := control.LogLevel(*level)
		if logLevel != control.LogDebug && logLevel != control.LogInfo && logLevel != control.LogWarn && logLevel != control.LogError {
			return logsUsage(stderr)
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		daemon, err := client.DialUnix(ctx, *socket)
		cancel()
		if err != nil {
			fmt.Fprintf(stderr, "logs: %v\n", err)
			return cli.ExitUnavailable
		}
		defer daemon.Close()
		if len(remaining) > 1 && remaining[1] == "clear" {
			ctx, cancel = context.WithTimeout(context.Background(), *timeout)
			defer cancel()
			return cli.ClearLogs(ctx, daemon, stdout, stderr)
		}
		return cli.Logs(context.Background(), daemon, uint32(*tail), logLevel, *follow, *jsonOutput, stdout, stderr)
	}
	if len(remaining) >= 2 && remaining[0] == "outbound" {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		daemon, err := client.DialUnix(ctx, *socket)
		cancel()
		if err != nil {
			fmt.Fprintf(stderr, "outbound: %v\n", err)
			return cli.ExitUnavailable
		}
		defer daemon.Close()
		ctx, cancel = context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		switch {
		case len(remaining) == 2 && remaining[1] == "list":
			return cli.OutboundList(ctx, daemon, *jsonOutput, stdout, stderr)
		case len(remaining) == 4 && remaining[1] == "select":
			return cli.OutboundSelect(ctx, daemon, remaining[2], remaining[3], stdout, stderr)
		case len(remaining) == 3 && remaining[1] == "test":
			scope := control.TestScope{}
			if remaining[2] == "all" {
				scope.AllVisible = true
			} else {
				scope.OutboundID = remaining[2]
			}
			return cli.OutboundTest(ctx, daemon, scope, stdout, stderr)
		case len(remaining) == 4 && remaining[1] == "test" && remaining[2] == "group":
			return cli.OutboundTest(ctx, daemon, control.TestScope{GroupID: remaining[3]}, stdout, stderr)
		default:
			return outboundUsage(stderr)
		}
	}
	if len(remaining) >= 1 && (remaining[0] == "connect" || remaining[0] == "disconnect" || remaining[0] == "restart") {
		command := remaining[0]
		connectionFlags := flag.NewFlagSet(command, flag.ContinueOnError)
		connectionFlags.SetOutput(stderr)
		profileID := connectionFlags.String("profile", "", "profile ID")
		mode := connectionFlags.String("mode", "", "tun, system-proxy, or local-proxy")
		if err := connectionFlags.Parse(remaining[1:]); err != nil || connectionFlags.NArg() != 0 || (command != "connect" && (*profileID != "" || *mode != "")) {
			return connectionUsage(stderr)
		}
		connectionMode := control.ConnectionMode(*mode)
		if command == "connect" && *mode != "" && connectionMode != control.ModeTUN && connectionMode != control.ModeSystemProxy && connectionMode != control.ModeLocalProxy {
			return connectionUsage(stderr)
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		daemon, err := client.DialUnix(ctx, *socket)
		cancel()
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", command, err)
			return cli.ExitUnavailable
		}
		defer daemon.Close()
		ctx, cancel = context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		return cli.ConnectionOperation(ctx, daemon, command, *profileID, connectionMode, *jsonOutput, stdout, stderr)
	}
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

func serviceUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui autoconnect status|enable|disable | service status | agent status | diagnostics")
	return cli.ExitUsage
}

func settingsUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui settings show|validate FILE|set FILE|import FILE|reset --yes|export [--include-secrets --yes]")
	return cli.ExitUsage
}

func logsUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] logs [--follow] [--level debug|info|warn|error] [--tail N] | logs clear --yes")
	return cli.ExitUsage
}

func outboundUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] outbound list|select GROUP_ID OUTBOUND_ID|test OUTBOUND_ID|test group GROUP_ID|test all")
	return cli.ExitUsage
}

func connectionUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] connect [--profile ID] [--mode tun|system-proxy|local-proxy] | disconnect | restart")
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
