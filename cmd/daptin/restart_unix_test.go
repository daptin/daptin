//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRestartProcessPreservesPIDArgumentsAndEnvironment(t *testing.T) {
	const stage = "DAPTIN_TEST_RESTART_STAGE"
	if os.Getenv(stage) == "replace" {
		if err := os.Setenv(stage, "replaced"); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("before=%d\n", os.Getpid())
		if err := restartProcess(); err != nil {
			t.Fatal(err)
		}
		t.Fatal("process replacement returned")
	}
	if os.Getenv(stage) == "replaced" {
		fmt.Printf("after=%d\n", os.Getpid())
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestRestartProcessPreservesPIDArgumentsAndEnvironment$")
	cmd.Env = append(os.Environ(), stage+"=replace")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restart subprocess: %v\n%s", err, output)
	}
	for _, expected := range []string{
		fmt.Sprintf("before=%d", cmd.Process.Pid),
		fmt.Sprintf("after=%d", cmd.Process.Pid),
	} {
		if !strings.Contains(string(output), expected) {
			t.Fatalf("missing %q in restarted process output:\n%s", expected, output)
		}
	}
}
