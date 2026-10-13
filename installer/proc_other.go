//go:build !windows

package main

import "os/exec"

func hideConsole(cmd *exec.Cmd) {}

// Off Windows (tests only) there is no signature check or Installed apps list.
func verifySignature(file string) error   { return nil }
func isInstalled(displayName string) bool { return false }
