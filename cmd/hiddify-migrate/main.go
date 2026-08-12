// hiddify-migrate produces a read-only migration plan from Hiddify GUI data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/1suo/hiddify-tui/internal/migrate"
)

func main() {
	flags := flag.NewFlagSet("hiddify-migrate", flag.ExitOnError)
	database := flags.String("database", "", "path to the Hiddify GUI SQLite db")
	configs := flags.String("configs", "", "path to the GUI configs directory")
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
	if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
		fmt.Fprintf(os.Stderr, "hiddify-migrate: %v\n", err)
		os.Exit(1)
	}
}
