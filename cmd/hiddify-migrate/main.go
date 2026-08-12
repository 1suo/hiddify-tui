// hiddify-migrate produces a read-only migration plan from Hiddify GUI data.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/migrate"
)

func main() {
	flags := flag.NewFlagSet("hiddify-migrate", flag.ExitOnError)
	database := flags.String("database", "", "path to the Hiddify GUI SQLite db")
	configs := flags.String("configs", "", "path to the GUI configs directory")
	apply := flags.Bool("apply", false, "import the reviewed plan into the daemon")
	yes := flags.Bool("yes", false, "confirm profile creation in the daemon")
	guiExited := flags.Bool("gui-exited", false, "confirm the GUI and its core are stopped")
	socket := flags.String("socket", client.DefaultSocket(), "daemon control socket")
	flags.Parse(os.Args[1:])
	if *database == "" || *configs == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: hiddify-migrate --database PATH --configs PATH")
		os.Exit(2)
	}
	plan, err := migrate.ReadPlan(*database, *configs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hiddify-migrate: %v\n", err)
		os.Exit(1)
	}
	if !*apply {
		if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
			fmt.Fprintf(os.Stderr, "hiddify-migrate: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if !*yes || !*guiExited {
		fmt.Fprintln(os.Stderr, "hiddify-migrate: --apply requires --yes and --gui-exited")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	daemon, err := client.DialUnix(ctx, *socket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hiddify-migrate: connect daemon: %v\n", err)
		os.Exit(1)
	}
	defer daemon.Close()
	result := migrate.Apply(ctx, plan, daemon)
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "hiddify-migrate: %v\n", err)
		os.Exit(1)
	}
}
