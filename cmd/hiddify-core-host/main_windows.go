//go:build windows

// hiddify-core-host exposes the official Windows core DLL as the same
// long-running process interface used by hiddify-core on Unix.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

const grpcNormalInsecure = 3

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "run" && os.Args[1] != "serve") {
		fmt.Fprintln(os.Stderr, "usage: hiddify-core-host <serve|run> -D STATE_DIR [-c CONFIG]")
		os.Exit(2)
	}

	command := os.Args[1]
	runFlags := flag.NewFlagSet(command, flag.ContinueOnError)
	configPath := runFlags.String("c", "", "bootstrap configuration")
	stateDir := runFlags.String("D", "", "core state directory")
	if err := runFlags.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if *stateDir == "" || (command == "run" && *configPath == "") {
		fmt.Fprintln(os.Stderr, "hiddify-core-host: -D is required; run also requires -c")
		os.Exit(2)
	}
	if err := os.MkdirAll(*stateDir, 0o700); err != nil {
		fatal("create state directory", err)
	}

	executable, err := os.Executable()
	if err != nil {
		fatal("resolve executable", err)
	}
	library := syscall.NewLazyDLL(filepath.Join(filepath.Dir(executable), "hiddify-core.dll"))
	setup := library.NewProc("setup")
	start := library.NewProc("start")
	closeGRPC := library.NewProc("closeGrpc")
	freeString := library.NewProc("freeString")
	if err := library.Load(); err != nil {
		fatal("load hiddify-core.dll", err)
	}

	state := cString(*stateDir)
	temp := cString(os.TempDir())
	listen := cString("127.0.0.1:17078")
	empty := cString("")
	result, _, _ := setup.Call(
		ptr(state), ptr(state), ptr(temp), grpcNormalInsecure,
		ptr(listen), ptr(empty), 0, 0,
	)
	runtime.KeepAlive(state)
	runtime.KeepAlive(temp)
	runtime.KeepAlive(listen)
	runtime.KeepAlive(empty)
	checkResult("setup core", result, freeString)

	if command == "run" {
		config := cString(*configPath)
		result, _, _ = start.Call(ptr(config), 0)
		runtime.KeepAlive(config)
		checkResult("start bootstrap profile", result, freeString)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	<-signals
	closeGRPC.Call(grpcNormalInsecure)
}

func cString(value string) *byte {
	result, err := syscall.BytePtrFromString(value)
	if err != nil {
		fatal("encode DLL argument", err)
	}
	return result
}

func ptr(value *byte) uintptr { return uintptr(unsafe.Pointer(value)) }

func checkResult(operation string, result uintptr, freeString *syscall.LazyProc) {
	if result == 0 {
		fatal(operation, fmt.Errorf("core returned a null result"))
	}
	message := readCString(result)
	freeString.Call(result)
	if message != "" {
		fatal(operation, fmt.Errorf("%s", message))
	}
}

func readCString(address uintptr) string {
	const maxResultLength = 64 << 10
	bytes := make([]byte, 0, 128)
	for offset := uintptr(0); offset < maxResultLength; offset++ {
		value := *(*byte)(unsafe.Pointer(address + offset))
		if value == 0 {
			return string(bytes)
		}
		bytes = append(bytes, value)
	}
	return "core result exceeded 64 KiB"
}

func fatal(operation string, err error) {
	fmt.Fprintf(os.Stderr, "hiddify-core-host: %s: %v\n", operation, err)
	os.Exit(1)
}
