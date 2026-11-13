//go:build !windows

package main

import (
	"os"
	"syscall"
)

// Replace the drained process, preserving PID 1 in containers and all startup
// arguments and environment. No external supervisor is needed for onboarding.
func restartProcess() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(executable, os.Args, os.Environ())
}
