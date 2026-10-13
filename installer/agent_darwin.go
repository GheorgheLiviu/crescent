package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// On a Mac, a crescent:// link reaches an app as an Apple event, not on the command line, and a Go
// program can't receive that without cgo. So setting up the agent also makes "Crescent Agent.app",
// a tiny AppleScript app (built by the Mac's own osacompile) that registers crescent:// and passes
// each link to this program.

const lsregister = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

// agentDir holds the agent program and its AppleScript app.
func agentDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Crescent")
}

// agentPath is the installed agent program, which the AppleScript app starts.
func agentPath() string { return filepath.Join(agentDir(), "Crescent") }

// appletSource is the AppleScript app's code: it starts the agent with the link, in the background,
// and quits.
func appletSource(agent string) string {
	q := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(agent) + `"`
	return "on open location u\n" +
		"\tdo shell script quoted form of " + q + " & \" \" & quoted form of u & \" > /dev/null 2>&1 &\"\n" +
		"end open location\n"
}

const appletURLTypes = `<array><dict><key>CFBundleURLName</key><string>Crescent</string>` +
	`<key>CFBundleURLSchemes</key><array><string>crescent</string></array></dict></array>`

// registerAgent runs when Crescent.app has no app list: it copies the program to agentDir and
// makes the AppleScript app that answers crescent:// links.
func registerAgent() {
	if err := setUpAgent(); err != nil {
		alert("Couldn't set up Crescent: " + err.Error())
		return
	}
	notice("Crescent is ready.\n\nGo back to the Crescent page and press Install on this Mac.")
}

func setUpAgent() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	target := agentPath()
	if err := os.MkdirAll(agentDir(), 0o755); err != nil {
		return err
	}
	if self != target {
		data, err := os.ReadFile(self)
		if err != nil {
			return err
		}
		if _, statErr := os.Stat(target); statErr == nil {
			err = (&agent{self: target}).update(data) // an older agent may be running
		} else {
			err = os.WriteFile(target, data, 0o755)
		}
		if err != nil {
			return err
		}
	}
	src := filepath.Join(agentDir(), "agent.applescript")
	if err := os.WriteFile(src, []byte(appletSource(target)), 0o644); err != nil {
		return err
	}
	defer os.Remove(src)
	app := filepath.Join(agentDir(), "Crescent Agent.app")
	os.RemoveAll(app)
	plist := filepath.Join(app, "Contents", "Info.plist")
	steps := [][]string{
		{"/usr/bin/osacompile", "-o", app, src},
		{"/usr/bin/plutil", "-replace", "CFBundleIdentifier", "-string", "com.crescent.agent", plist},
		{"/usr/bin/plutil", "-replace", "CFBundleURLTypes", "-xml", appletURLTypes, plist},
		{"/usr/bin/plutil", "-replace", "LSUIElement", "-bool", "YES", plist},
		// Editing Info.plist breaks the app's signature; sign it again ad hoc, as osacompile did.
		{"/usr/bin/codesign", "--force", "--sign", "-", app},
		{lsregister, "-f", app},
	}
	for _, s := range steps {
		if out, err := exec.Command(s[0], s[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %v: %s", filepath.Base(s[0]), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// runAgent is started by a crescent://install?token=… link: it waits for the page to send the app
// list, installs it, and keeps answering the page until it has seen the result.
func runAgent(link string) {
	token := linkToken(link)
	if token == "" {
		alert("This Crescent link is damaged. Press Install on the Crescent page again.")
		return
	}
	defer setupEnv()()
	in := &Installer{Brew: findBrew()}
	a := newAgent(token, in, func(a *agent, p *Payload, apps []App) any {
		r := installAll(in, p.AutoUpdate, apps, a)
		for i := range p.Scripts {
			if in.cancelled.Load() {
				break
			}
			note, _ := in.RunScript(&p.Scripts[i], a)
			r.Notes = append(r.Notes, note)
		}
		return agentResult(r)
	})
	// Only the installed copy updates itself; a Mac lets the running program be replaced.
	if self, err := os.Executable(); err == nil && self == agentPath() {
		a.self = self
		os.Remove(self + ".old") // left by the last update
	}
	if a.serve() != nil {
		alert("Crescent is already installing. Wait for it to finish, then try again.")
	}
}
