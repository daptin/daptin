package main

import "os"

// Windows cannot replace a running process. Start the same executable after
// draining; main returns once the replacement has started.
func restartProcess() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	process, err := os.StartProcess(executable, os.Args, &os.ProcAttr{
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return err
	}
	return process.Release()
}
