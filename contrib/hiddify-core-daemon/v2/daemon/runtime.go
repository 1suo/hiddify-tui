// Package daemon owns the lifecycle primitives for the always-running desktop
// daemon. Networking/core initialization is supplied by the caller after the
// process lock and local control socket have been acquired.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

type Runtime struct {
	listener net.Listener
	lock     *os.File
	socket   string
}

// Options controls local Unix socket ownership and peer admission. An allowed
// UID of -1 retains the development default (filesystem permissions only).
type Options struct {
	AllowedUID int
}

// Start acquires the daemon-owned state lock and creates a local Unix control
// socket. A running daemon is never replaced, and stale socket files are
// removed only after a listener has been proven unavailable.
func Start(stateDir, socket string) (*Runtime, error) {
	return StartWithOptions(stateDir, socket, Options{AllowedUID: -1})
}

func StartWithOptions(stateDir, socket string, options Options) (*Runtime, error) {
	if stateDir == "" || socket == "" {
		return nil, errors.New("state directory and socket path are required")
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(stateDir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open daemon lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("another hiddify daemon owns %s: %w", stateDir, err)
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0750); err != nil {
		lock.Close()
		return nil, fmt.Errorf("create socket directory: %w", err)
	}
	if err := removeStaleSocket(socket); err != nil {
		lock.Close()
		return nil, err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		lock.Close()
		return nil, fmt.Errorf("listen on control socket: %w", err)
	}
	if options.AllowedUID >= 0 {
		if err := os.Chown(socket, options.AllowedUID, -1); err != nil {
			listener.Close()
			os.Remove(socket)
			lock.Close()
			return nil, fmt.Errorf("set control socket owner: %w", err)
		}
	}
	if err := os.Chmod(socket, socketMode(options.AllowedUID)); err != nil {
		listener.Close()
		os.<truncated omitted_approx_tokens=
