//go:build !windows && !darwin

package main

import (
	"os"
	"os/exec"
)

// Off Windows (tests only) a batch script runs with sh and a PowerShell one with pwsh.
var scriptCommand = func(file, kind string) *exec.Cmd {
	if kind == "ps1" {
		return exec.Command("pwsh", "-NoProfile", "-File", file)
	}
	return exec.Command("sh", file)
}

// Like Windows: PowerShell and batch scripts, each with a window of its own.
var scriptKinds = map[string]bool{"ps1": true, "cmd": true}

const scriptOwnWindow = true

func startScript(cmd *exec.Cmd, title string) error { return cmd.Start() }

func freshEnv() []string { return os.Environ() }
