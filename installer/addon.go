package main

// Add-ons for Claude Code: graphify (a knowledge graph of your code) and Ponytail (a lazy senior
// dev mode). They aren't apps winget or Homebrew know, so the installer sets them up itself, for
// the signed-in user, after the apps they need (Claude Code, uv, Git), with no window of their own.

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

// addonStep is one command; when mayFail is set, its failure doesn't stop the add-on.
type addonStep struct {
	args    []string
	mayFail bool
}

var addons = map[string][]addonStep{
	// graphify is the PyPI package graphifyy; uv brings its own Python. "graphify install" puts
	// the /graphify skill in ~/.claude/skills and turns it on in ~/.claude/CLAUDE.md.
	"Crescent.Graphify": {
		{args: []string{"uv", "tool", "install", "--upgrade", "graphifyy"}},
		{args: []string{"uv", "tool", "update-shell"}, mayFail: true}, // puts graphify on the Path for terminals
		{args: []string{"graphify", "install"}},
	},
	// Ponytail is a Claude Code plugin from its own marketplace; adding the marketplace a second
	// time fails, which is fine.
	"Crescent.Ponytail": {
		{args: []string{"claude", "plugin", "marketplace", "add", "DietrichGebert/ponytail"}, mayFail: true},
		{args: []string{"claude", "plugin", "install", "ponytail@ponytail"}},
	},
}

func isAddon(id string) bool { _, ok := addons[id]; return ok }

// lookPathIn finds a program on the Path of env (the installer's own Path may be older).
func lookPathIn(env []string, name string) (string, error) {
	for _, kv := range env {
		if k, v, _ := strings.Cut(kv, "="); strings.EqualFold(k, "PATH") {
			for _, dir := range filepath.SplitList(v) {
				if p, err := exec.LookPath(filepath.Join(dir, name)); err == nil && dir != "" {
					return p, nil
				}
			}
		}
	}
	return "", errors.New(name + " wasn't found")
}

// addonProgram finds a program the apps installed a moment ago: on the Path a new window would
// get, then in uv's tool folder (graphify lands there).
func addonProgram(env []string, name string) (string, error) {
	p, err := lookPathIn(env, name)
	if err == nil || name == "uv" {
		return p, err
	}
	if uv, uvErr := lookPathIn(env, "uv"); uvErr == nil {
		cmd := exec.Command(uv, "tool", "dir", "--bin")
		cmd.Env = env
		hideConsole(cmd)
		if out, outErr := cmd.Output(); outErr == nil {
			if p, binErr := exec.LookPath(filepath.Join(strings.TrimSpace(string(out)), name)); binErr == nil {
				return p, nil
			}
		}
	}
	return "", err
}

func (in *Installer) runAddon(i int, a App, ui UI, r *Result) {
	var err error
	for _, s := range addons[a.ID] {
		if in.cancelled.Load() {
			break
		}
		env := freshEnv() // again for each step: uv tool update-shell changes the Path
		var prog string
		if prog, err = addonProgram(env, s.args[0]); err == nil {
			cmd := exec.Command(prog, s.args[1:]...)
			cmd.Dir, cmd.Env = homeDir(), env
			hideConsole(cmd)
			err = in.runLogged(cmd, ui.Log)
		}
		if err != nil && !s.mayFail {
			break
		}
		err = nil
	}
	switch {
	case in.cancelled.Load():
		ui.Log("Cancelled.")
		ui.Outcome(i, "cancelled")
		r.Cancelled = true
	case err == nil:
		r.Installed++
		ui.Outcome(i, "installed")
	default:
		ui.Log("Failed: " + err.Error())
		ui.Outcome(i, "failed")
		r.Failed = append(r.Failed, a.Name)
	}
}
