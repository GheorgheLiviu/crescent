package main

// Homebrew, the Mac's winget. In the Mac build each app's ID is its Homebrew package, such as
// cask:google-chrome, formula:node@22 or a plain name, and "brew install" installs it.

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// brewPackage matches a Homebrew package: an optional cask: or formula:, then a name, which may
// be a tap's package (mongodb/brew/mongodb-community).
var brewPackage = regexp.MustCompile(`^((cask|formula):)?[a-z0-9][a-z0-9@+._-]*(/[a-z0-9][a-z0-9@+._-]*){0,2}$`)

func validBrew(id string) bool {
	return len(id) <= 100 && brewPackage.MatchString(id) && !strings.Contains(id, "..")
}

// brewArgs is the install command for a package; a plain name lets Homebrew pick formula or cask.
func brewArgs(id string) []string {
	if kind, name, ok := strings.Cut(id, ":"); ok {
		return []string{"install", "--" + kind, name}
	}
	return []string{"install", id}
}

// brewAlready matches Homebrew's lines for an app that is already there: installed with
// Homebrew, or put in Applications some other way (then brew stops with an error).
var brewAlready = regexp.MustCompile(`is already installed|already an App at|already a Binary at`)

func (in *Installer) runBrew(i int, a App, ui UI, r *Result) {
	already := false
	err := in.runLogged(exec.Command(in.Brew, brewArgs(a.ID)...), func(line string) {
		already = already || brewAlready.MatchString(line)
		ui.Log(line)
	})
	switch {
	case in.cancelled.Load():
		ui.Log("Cancelled.")
		ui.Outcome(i, "cancelled")
		r.Cancelled = true
	case already:
		ui.Log("Already installed, skipped.")
		ui.Outcome(i, "skipped")
		r.Skipped++
	case err == nil:
		r.Installed++
		ui.Outcome(i, "installed")
	default:
		ui.Log("Failed: " + err.Error())
		ui.Outcome(i, "failed")
		r.Failed = append(r.Failed, a.Name)
	}
}

const brewUpdatesLabel = "com.crescent.updates"

// brewUpdatesPlist is a launchd job that upgrades every Homebrew app on Sundays at 12:00; launchd
// runs a missed one when the Mac wakes.
func brewUpdatesPlist(brew string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(brew))
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>` + brewUpdatesLabel + `</string>
  <key>ProgramArguments</key><array><string>` + b.String() + `</string><string>upgrade</string></array>
  <key>EnvironmentVariables</key><dict><key>HOMEBREW_NO_ENV_HINTS</key><string>1</string></dict>
  <key>StartCalendarInterval</key><dict><key>Weekday</key><integer>0</integer><key>Hour</key><integer>12</integer><key>Minute</key><integer>0</integer></dict>
</dict>
</plist>
`
}

// scheduleBrewUpdates adds the weekly upgrade for this user and returns where its file is.
func scheduleBrewUpdates(brew string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, "Library", "LaunchAgents", brewUpdatesLabel+".plist")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(brewUpdatesPlist(brew)), 0o644); err != nil {
		return "", err
	}
	exec.Command("launchctl", "unload", path).Run() // an older copy, if any
	if out, err := exec.Command("launchctl", "load", "-w", path).CombinedOutput(); err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return path, nil
}
