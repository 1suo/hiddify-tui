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
	"github.com/1suo/hiddify-tui/internal/migrate"
	"github.com/1suo/hiddify-tui/internal/profile"
	"github.com/1suo/hiddify-tui/internal/tui"
	"github.com/charmbracelet/x/term"
)

// version is overridable at build time with -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func main() {
	if address, timeout, noColor, ok := tuiInvocation(os.Args[1:]); ok {
		var core client.Client
		dialed, dialErr := cli.Dial(address, timeout)
		if dialErr == nil {
			core = dialed
			defer dialed.Close()
		}
		store, storeErr := profile.Open(profile.DefaultPath())
		if storeErr != nil {
			fmt.Fprintf(os.Stderr, "profiles: %v\n", storeErr)
			os.Exit(cli.ExitRejected)
		}
		if err := tui.RunWithOptions(core, store, dialErr, noColor); err != nil {
			fmt.Fprintf(os.Stderr, "tui: %v\n", err)
			os.Exit(cli.ExitRejected)
		}
		return
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func tuiInvocation(args []string) (string, time.Duration, bool, bool) {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return "", 0, false, false
	}
	flags := flag.NewFlagSet("hiddify-tui", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	address := flags.String("address", client.DefaultAddress, "core gRPC address")
	timeout := flags.Duration("timeout", 3*time.Second, "core request timeout")
	jsonOutput := flags.Bool("json", false, "print JSON")
	showVersion := flags.Bool("version", false, "print version")
	noColor := flags.Bool("no-color", false, "disable terminal colors")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *jsonOutput || *showVersion {
		return "", 0, false, false
	}
	return *address, *timeout, *noColor, true
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("hiddify-tui", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print JSON")
	address := flags.String("address", client.DefaultAddress, "core gRPC address")
	timeout := flags.Duration("timeout", 3*time.Second, "core request timeout")
	profileFile := flags.String("profile-file", profile.DefaultPath(), "client profile store path")
	showVersion := flags.Bool("version", false, "print version")
	_ = flags.Bool("no-color", false, "disable terminal colors")
	if err := flags.Parse(args); err != nil {
		return cli.ExitUsage
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return cli.ExitOK
	}

	remaining := flags.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(stderr, "usage: hiddify-tui [--json] status")
		return cli.ExitUsage
	}

	command := remaining[0]
	store, storeErr := cli.ProfileStore(*profileFile)
	if storeErr != nil {
		cli.WriteError(stderr, command, storeErr)
		return cli.ExitRejected
	}

	// Commands that do not need a live core.
	switch command {
	case "profile":
		return runProfile(remaining, store, *jsonOutput, stdout, stderr)
	case "migrate":
		return runGUIMigration(remaining, store, stdout, stderr)
	}

	core, err := cli.Dial(*address, *timeout)
	if err != nil {
		cli.WriteError(stderr, command, err)
		return cli.ExitUnavailable
	}
	defer core.Close()

	switch command {
	case "status":
		return runStatus(remaining, core, *jsonOutput, stdout, stderr)
	case "connect", "disconnect", "restart":
		return runConnection(remaining, core, store, *jsonOutput, stdout, stderr)
	case "outbound":
		return runOutbound(remaining, core, *jsonOutput, stdout, stderr)
	case "logs":
		return runLogs(remaining, core, *jsonOutput, stdout, stderr)
	case "settings":
		return runSettings(remaining, core, stdout, stderr)
	default:
		fmt.Fprintln(stderr, "usage: hiddify-tui [--json] status")
		return cli.ExitUsage
	}
}

func runStatus(args []string, core client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	ctx := context.Background()
	if len(args) >= 2 && args[1] == "--watch" {
		return cli.StatusWatch(ctx, core, jsonOutput, stdout, stderr)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return cli.Status(ctx, core, jsonOutput, stdout, stderr)
}

func runConnection(args []string, core client.Client, store *profile.Store, jsonOutput bool, stdout, stderr io.Writer) int {
	command := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	switch command {
	case "connect":
		profileID := ""
		flags := flag.NewFlagSet("connect", flag.ContinueOnError)
		flags.SetOutput(stderr)
		flags.StringVar(&profileID, "profile", "", "profile ID")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
			return connectionUsage(stderr)
		}
		return cli.Connect(ctx, core, store, profileID, jsonOutput, stdout, stderr)
	case "disconnect":
		return cli.Disconnect(ctx, core, jsonOutput, stdout, stderr)
	case "restart":
		return cli.Restart(ctx, core, store, jsonOutput, stdout, stderr)
	}
	return cli.ExitUsage
}

func runOutbound(args []string, core client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if len(args) < 2 {
		return outboundUsage(stderr)
	}
	switch {
	case args[1] == "list" && len(args) == 2:
		return cli.OutboundList(ctx, core, jsonOutput, stdout, stderr)
	case args[1] == "select" && len(args) == 4:
		return cli.OutboundSelect(ctx, core, args[2], args[3], stdout, stderr)
	case args[1] == "test" && len(args) == 3:
		return cli.OutboundTest(ctx, core, args[2], stdout, stderr)
	default:
		return outboundUsage(stderr)
	}
}

func runLogs(args []string, core client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("logs", flag.ContinueOnError)
	flags.SetOutput(stderr)
	level := flags.String("level", "info", "debug, info, warn, or error")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return logsUsage(stderr)
	}
	logLevel := client.LogLevel(*level)
	switch logLevel {
	case client.LogDebug, client.LogInfo, client.LogWarn, client.LogError:
	default:
		return logsUsage(stderr)
	}
	return cli.Logs(context.Background(), core, logLevel, jsonOutput, stdout, stderr)
}

func runSettings(args []string, core client.Client, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return settingsUsage(stderr)
	}
	command := args[1]
	if (command == "set" || command == "validate") && len(args) == 3 {
		data, err := os.ReadFile(args[2])
		if err != nil {
			cli.WriteError(stderr, "settings "+command, err)
			return cli.ExitUsage
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if command == "set" {
			return cli.SettingsSet(ctx, core, data, stdout, stderr)
		}
		return cli.SettingsValidate(ctx, core, data, stdout, stderr)
	}
	return settingsUsage(stderr)
}

func runProfile(args []string, store *profile.Store, jsonOutput bool, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return profileUsage(stderr)
	}
	command := args[1]
	switch command {
	case "list":
		if len(args) != 2 {
			return profileUsage(stderr)
		}
		return cli.ProfileList(store, jsonOutput, stdout, stderr)
	case "show":
		if len(args) != 3 {
			return profileUsage(stderr)
		}
		return cli.ProfileShow(store, args[2], jsonOutput, stdout, stderr)
	case "activate":
		if len(args) != 3 {
			return profileUsage(stderr)
		}
		return cli.ProfileActivate(store, args[2], jsonOutput, stdout, stderr)
	case "rename":
		if len(args) != 4 {
			return profileUsage(stderr)
		}
		return cli.ProfileRename(store, args[2], args[3], jsonOutput, stdout, stderr)
	case "delete":
		if len(args) != 4 || args[3] != "--yes" {
			fmt.Fprintln(stderr, "profile delete requires --yes")
			return cli.ExitUsage
		}
		return cli.ProfileDelete(store, args[2], jsonOutput, stdout, stderr)
	}
	// Commands that need a live core (add/refresh validate via Parse).
	core, err := cli.Dial(client.DefaultAddress, 10*time.Second)
	if err != nil {
		cli.WriteError(stderr, "profile "+command, err)
		return cli.ExitUnavailable
	}
	defer core.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	switch command {
	case "add":
		flags := flag.NewFlagSet("profile add", flag.ContinueOnError)
		flags.SetOutput(stderr)
		name := flags.String("name", "", "profile name")
		active := flags.Bool("activate", false, "make profile active")
		if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 1 {
			return profileUsage(stderr)
		}
		return cli.ProfileAddRemote(ctx, core, store, flags.Arg(0), *name, *active, jsonOutput, stdout, stderr)
	case "add-file", "add-stdin":
		flags := flag.NewFlagSet("profile "+command, flag.ContinueOnError)
		flags.SetOutput(stderr)
		name := flags.String("name", "", "profile name")
		active := flags.Bool("activate", false, "make profile active")
		if err := flags.Parse(args[2:]); err != nil || (command == "add-file" && flags.NArg() != 1) || (command == "add-stdin" && flags.NArg() != 0) {
			return profileUsage(stderr)
		}
		var content io.Reader = os.Stdin
		if command == "add-file" {
			file, err := os.Open(flags.Arg(0))
			if err != nil {
				cli.WriteError(stderr, "profile add-file", err)
				return cli.ExitUsage
			}
			defer file.Close()
			content = file
			if *name == "" {
				*name = filepath.Base(flags.Arg(0))
			}
		}
		data, err := io.ReadAll(content)
		if err != nil {
			cli.WriteError(stderr, "profile "+command, err)
			return cli.ExitRejected
		}
		return cli.ProfileAddLocal(ctx, core, store, string(data), *name, *active, jsonOutput, stdout, stderr)
	case "refresh":
		if len(args) != 3 {
			return profileUsage(stderr)
		}
		return cli.ProfileRefresh(ctx, core, store, args[2], jsonOutput, stdout, stderr)
	default:
		return profileUsage(stderr)
	}
}

func runGUIMigration(args []string, store *profile.Store, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[1] != "gui" {
		return migrationUsage(stderr)
	}
	flags := flag.NewFlagSet("migrate gui", flag.ContinueOnError)
	flags.SetOutput(stderr)
	database := flags.String("database", "", "path to the Hiddify GUI SQLite database")
	configs := flags.String("configs", "", "path to the Hiddify GUI config directory")
	dryRun := flags.Bool("dry-run", false, "print the read-only migration plan")
	apply := flags.Bool("apply", false, "import the reviewed plan into the profile store")
	if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 || *database == "" || *configs == "" || (*dryRun && *apply) {
		return migrationUsage(stderr)
	}
	plan, err := migrate.ReadPlan(*database, *configs)
	if err != nil {
		cli.WriteError(stderr, "migrate gui", err)
		return cli.ExitRejected
	}
	if *dryRun || !*apply {
		if err := json.NewEncoder(stdout).Encode(plan); err != nil {
			return cli.ExitRejected
		}
		return cli.ExitOK
	}
	result := migrate.Apply(context.Background(), plan, store)
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return cli.ExitRejected
	}
	return cli.ExitOK
}

func serviceUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui status [--watch] | connect | disconnect | restart")
	return cli.ExitUsage
}

func settingsUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui settings set FILE | settings validate FILE")
	return cli.ExitUsage
}

func logsUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] logs [--level debug|info|warn|error]")
	return cli.ExitUsage
}

func outboundUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] outbound list|select GROUP OUTBOUND|test OUTBOUND")
	return cli.ExitUsage
}

func connectionUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] connect [--profile ID] | disconnect | restart")
	return cli.ExitUsage
}

func profileUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui [--json] profile list|show ID|add [--name NAME] [--activate] URL|add-file [--name NAME] [--activate] FILE|add-stdin [--name NAME] [--activate]|rename ID NAME|activate ID|refresh ID|delete ID --yes")
	return cli.ExitUsage
}

func migrationUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: hiddify-tui migrate gui --database PATH --configs DIR [--dry-run|--apply]")
	return cli.ExitUsage
}
