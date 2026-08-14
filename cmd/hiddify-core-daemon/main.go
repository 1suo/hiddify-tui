// hiddify-core-daemon keeps the standalone core's gRPC control service alive
// independently of the TUI. The networking engine remains controllable through
// the Core service and the daemon follows the core's process lifetime.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/core"
)

func main() {
	coreBinary := flag.String("core-binary", "", "path to hiddify-core")
	address := flag.String("address", client.DefaultAddress, "core gRPC address")
	stateDir := flag.String("state-dir", "", "core state directory")
	timeout := flag.Duration("timeout", 15*time.Second, "core startup timeout")
	flag.Parse()

	launcher := core.NewLauncher(*coreBinary)
	if *stateDir != "" {
		launcher.SetStateDir(*stateDir)
	}
	if !launcher.Available() {
		fmt.Fprintln(os.Stderr, "hiddify-core-daemon: hiddify-core binary not found")
		os.Exit(1)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	coreClient, err := launcher.Start(context.Background(), *address, core.BootstrapConfig, *timeout)
	if err != nil {
		if errors.Is(err, core.ErrAddressInUse) {
			fmt.Fprintf(os.Stderr, "hiddify-core-daemon: %v; leaving the existing core untouched\n", err)
			<-signals
			return
		}
		fmt.Fprintf(os.Stderr, "hiddify-core-daemon: %v\n", err)
		os.Exit(1)
	}
	defer coreClient.Close()

	select {
	case <-signals:
		launcher.Stop()
	case err := <-launcher.Done():
		fmt.Fprintf(os.Stderr, "hiddify-core-daemon: core exited: %v\n", err)
		os.Exit(1)
	}
}
