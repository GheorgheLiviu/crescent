package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// registerAgent runs when this program has no app list: it copies itself to
// %LOCALAPPDATA%\Crescent and registers crescent:// so the page's Install button can start it.
func registerAgent() {
	self, err := os.Executable()
	if err != nil {
		msgBox("Couldn't set up Crescent: "+err.Error(), mbOK|mbIconError)
		return
	}
	target := agentPath()
	dir := filepath.Dir(target)
	if !strings.EqualFold(self, target) {
		data, err := os.ReadFile(self)
		if err == nil {
			err = os.MkdirAll(dir, 0o755)
		}
		if err == nil {
			if _, statErr := os.Stat(target); statErr == nil {
				// An older agent may be running; swap it like an update instead of overwriting it.
				err = (&agent{self: target}).update(data)
			} else {
				err = os.WriteFile(target, data, 0o755)
			}
		}
		if err != nil {
			msgBox("Couldn't set up Crescent: "+err.Error(), mbOK|mbIconError)
			return
		}
	}
	set := func(path, name, value string) error {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+path, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(name, value)
	}
	err = set("crescent", "", "URL:Crescent")
	if err == nil {
		err = set("crescent", "URL Protocol", "")
	}
	if err == nil {
		err = set(`crescent\shell\open\command`, "", `"`+target+`" "%1"`)
	}
	if err != nil {
		msgBox("Couldn't set up Crescent: "+err.Error(), mbOK|mbIconError)
		return
	}
	msgBox("Crescent is ready.\n\nGo back to the Crescent page and press Install.", mbOK|mbIconInfo)
}

// agentPath is where the agent is installed and what crescent:// starts.
func agentPath() string { return filepath.Join(os.Getenv("LOCALAPPDATA"), "Crescent", "Crescent.exe") }

// runAgent is started by a crescent://install?token=… link: it waits for the page to send the app
// list, installs it, and keeps answering the page until it has seen the result.
func runAgent(link string) {
	token := linkToken(link)
	if token == "" {
		msgBox("This Crescent link is damaged. Press Install on the Crescent page again.", mbOK|mbIconError)
		return
	}
	winget, err := exec.LookPath("winget")
	if err != nil {
		if msgBox("This PC doesn't have winget (App Installer) yet, which Crescent needs.\n\nOpen the Microsoft Store to install it? Then press Install again.", mbYesNo|mbIconQuestion) == idYes {
			openStore()
		}
		return
	}
	in := &Installer{Winget: winget}
	a := newAgent(token, in, func(a *agent, p *Payload, apps []App) any {
		r := in.Run(apps, a)
		finish(winget, p, &r)
		for i := range p.Scripts {
			if in.cancelled.Load() {
				break
			}
			note, _ := in.RunScript(&p.Scripts[i], a)
			r.Notes = append(r.Notes, note)
		}
		return agentResult(r)
	})
	// Only the installed copy updates itself.
	if self, err := os.Executable(); err == nil && strings.EqualFold(self, agentPath()) {
		a.self = self
		os.Remove(self + ".old") // left by the last update
	}
	if a.serve() != nil {
		msgBox("Crescent is already installing. Wait for it to finish, then try again.", mbOK|mbIconInfo)
	}
}
