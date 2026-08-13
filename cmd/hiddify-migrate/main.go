// hiddify-migrate produces a read-only migration plan from Hiddify GUI data.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/1suo/hiddify-tui/internal/migrate"
	"github.com/1suo/hiddify-tui/internal/profile"
)

func main() {
	flags := flag.NewFlagSet("hiddify-migrate", flag.ExitOnError)
	database := flags.String("database", "", "path to the Hiddify GUI SQLite db")
	configs := flags.String("configs", "", "path to the GUI configs directory")
	apply := flags.Bool("apply", false, "import the reviewed plan into the profile store")
	storePath := flags.String("profile-file", profile.DefaultPath(), "client profile store path")
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
	store, err := profile.Open(*storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hiddify-migrate: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result := migrate.Apply(ctx, plan, store)
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "hiddify-migrate: %v\n", err)
		os.Exit(1)
	}
}
