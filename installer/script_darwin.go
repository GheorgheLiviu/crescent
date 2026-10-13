package main

import (
	"bytes"
	"os"
	"os/exec"
)

// On the Mac scripts are shell scripts. They run as the user, without a window; their output
// goes to the installer's details. "sudo" works through the password dialog (see main_darwin.go).
var scriptKinds = map[string]bool{"sh": true}

const scriptOwnWindow = false

// scriptCommand runs a script that names its shell (#!/bin/zsh) with that shell, others with bash.
var scriptCommand = func(file, kind string) *exec.Cmd {
	if head, err := os.ReadFile(file); err == nil && bytes.HasPrefix(head, []byte("#!")) && os.Chmod(file, 0o700) == nil {
		return exec.Command(file)
	}
	return exec.Command("/bin/bash", file)
}

func startScript(cmd *exec.Cmd, title string) error { return cmd.Start() }

// The environment main_darwin.go set up: Homebrew's folders on the PATH and the password dialog.
func freshEnv() []string { return os.Environ() }
