package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Script is one of the scripts ticked on the page (from the project's script library or the
// user's own). Scripts run after all apps are installed, one after another. On Windows each runs
// as administrator in a console window of its own; on the Mac it runs as the user, in the
// background, and its output goes to the installer's details.
type Script struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // "ps1" (PowerShell) or "cmd" (Command Prompt batch); "sh" on the Mac
	Body string `json:"body"`
}

const (
	maxScriptBytes = 3000000 // 1,000,000 characters on the page, even if each takes 3 bytes
	maxScripts     = 20      // the most scripts one installer runs
)

// cleanScripts returns the scripts ready to run, in the page's order: invalid ones are left out
// and at most maxScripts are kept. It returns nil when none is left.
func cleanScripts(list []Script) []Script {
	var out []Script
	for i := range list {
		if len(out) == maxScripts {
			break
		}
		if s := cleanScript(&list[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}

// scriptName is the file name rule the page uses too; the extension must also match the kind.
var scriptName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._()-]{0,59}$`)

// reservedName matches names Windows keeps for devices (CON.cmd would write to the console).
var reservedName = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`)

// cleanScript returns the script ready to run, or nil when there is none or it isn't valid.
func cleanScript(s *Script) *Script {
	if s == nil || !scriptKinds[s.Kind] {
		return nil
	}
	body := strings.TrimPrefix(s.Body, "\uFEFF")
	if strings.TrimSpace(body) == "" || len(body) > maxScriptBytes || strings.ContainsRune(body, 0) {
		return nil
	}
	name := s.Name
	if !validScriptName(name, s.Kind) {
		name = "script." + s.Kind
	}
	return &Script{Name: name, Kind: s.Kind, Body: body}
}

func validScriptName(name, kind string) bool {
	if !scriptName.MatchString(name) {
		return false
	}
	if base, _, _ := strings.Cut(name, "."); reservedName.MatchString(strings.TrimSpace(base)) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch kind {
	case "ps1":
		return ext == ".ps1"
	case "sh":
		return ext == ".sh"
	}
	return ext == ".cmd" || ext == ".bat"
}

// scriptLines counts the script's lines for the installer window; a final line break adds none.
func scriptLines(body string) int {
	return strings.Count(strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), "\n"), "\n") + 1
}

// scriptFile is the script as written to disk: Windows line endings, and UTF-8 that Windows
// PowerShell and Command Prompt both read correctly (accented letters, Romanian ă, ș, ț...).
// PowerShell 5 needs the byte order mark; a batch file instead switches its window to UTF-8
// first (cmd rereads the file line by line, so the lines after it are read as UTF-8).
func scriptFile(s *Script) []byte {
	if s.Kind == "sh" { // the Mac's shells want plain Unix line endings
		return []byte(strings.ReplaceAll(s.Body, "\r\n", "\n"))
	}
	body := strings.ReplaceAll(strings.ReplaceAll(s.Body, "\r\n", "\n"), "\n", "\r\n")
	if s.Kind == "ps1" {
		return []byte("\xEF\xBB\xBF" + body)
	}
	return []byte("@chcp 65001 >nul\r\n" + body)
}

// RunScript writes the script to a new temporary folder, runs it in its own window and waits
// for it. Like a maker's setup, the window is the user's: Cancel doesn't close it. It runs even
// after Cancel; the caller decides whether to run it.
func (in *Installer) RunScript(s *Script, ui UI) (note string, ok bool) {
	if s == nil {
		return "", false
	}
	ui.Log("--- Your script: " + s.Name + " ---")
	fail := func(err error) (string, bool) {
		note := fmt.Sprintf("Couldn't run your script %s: %v", s.Name, err)
		ui.Log(note)
		return note, false
	}
	dir, err := os.MkdirTemp("", "Crescent-script-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, s.Name)
	if err := os.WriteFile(file, scriptFile(s), 0o644); err != nil {
		return fail(err)
	}

	cmd := scriptCommand(file, s.Kind)
	cmd.Dir, cmd.Env = homeDir(), freshEnv()
	var out *lineWriter
	if !scriptOwnWindow {
		out = &lineWriter{log: ui.Log}
		cmd.Stdout, cmd.Stderr = out, out
	}
	in.mu.Lock()
	err = startScript(cmd, "Crescent: "+s.Name)
	if err == nil {
		in.current, in.manual = cmd, true
	}
	in.mu.Unlock()
	if err != nil {
		return fail(err)
	}
	if scriptOwnWindow {
		ui.Log("Its window is open. Crescent continues when it closes.")
	}
	err = cmd.Wait()
	if out != nil {
		out.flush()
	}
	in.mu.Lock()
	in.current, in.manual = nil, false
	in.mu.Unlock()

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		note = fmt.Sprintf("Your script %s finished.", s.Name)
		ok = true
	case errors.As(err, &exitErr):
		code := exitErr.ExitCode()
		if code > 0xFFFF { // Windows status codes, such as 0xC000013A when its window was closed
			note = fmt.Sprintf("Your script %s ended with exit code 0x%X.", s.Name, uint32(code))
		} else {
			note = fmt.Sprintf("Your script %s ended with exit code %d.", s.Name, code)
		}
	default:
		return fail(err)
	}
	ui.Log(note)
	return note, ok
}

// envVar is one variable from the registry; expand marks REG_EXPAND_SZ values.
type envVar struct {
	name, value string
	expand      bool
}

// samePerUser lists variables that belong to this sign-in rather than the registry keys (the
// machine key holds USERNAME=SYSTEM, which Windows replaces at sign-in).
var samePerUser = map[string]bool{"USERNAME": true, "USERDOMAIN": true, "USERPROFILE": true,
	"HOMEDRIVE": true, "HOMEPATH": true, "APPDATA": true, "LOCALAPPDATA": true}

// mergeEnv builds the environment a newly opened window would get: the process environment,
// then the machine and user variables over it (plain values before ones that refer to others,
// as Windows does), with Path the machine Path followed by the user Path. So a script sees
// what the apps just installed added, such as git or python on the Path. Names ignore case.
// homeDir is the user's home folder, where scripts and add-ons run, or the temp folder without one.
func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return os.TempDir()
}

func mergeEnv(base []string, machine, user []envVar) []string {
	env := newEnvSet(base)
	var paths [2]*envVar
	for i, vars := range [][]envVar{machine, user} {
		for _, expand := range []bool{false, true} {
			for j, v := range vars {
				switch {
				case v.expand != expand || v.name == "" || samePerUser[strings.ToUpper(v.name)]:
				case strings.EqualFold(v.name, "Path"):
					paths[i] = &vars[j]
				case expand:
					env.set(v.name, env.expand(v.value))
				default:
					env.set(v.name, v.value)
				}
			}
		}
	}
	if paths[0] == nil {
		return env.list() // no machine Path: keep this process's
	}
	// The Path is expanded last, so it can name variables an installer added next to it (NVM_HOME).
	var lists []string
	for _, p := range paths {
		switch {
		case p == nil:
		case p.expand:
			lists = append(lists, env.expand(p.value))
		default:
			lists = append(lists, p.value)
		}
	}
	env.set("Path", joinPath(lists...))
	return env.list()
}

// joinPath joins ;-separated folder lists, leaving out empty entries.
func joinPath(lists ...string) string {
	var parts []string
	for _, l := range lists {
		for _, p := range strings.Split(l, ";") {
			if p != "" {
				parts = append(parts, p)
			}
		}
	}
	return strings.Join(parts, ";")
}

// envSet is an environment whose names ignore case, as on Windows. It keeps the order and the
// first spelling of each name.
type envSet struct {
	names []string          // as first spelled
	vals  map[string]string // by upper-case name
}

func newEnvSet(environ []string) *envSet {
	e := &envSet{vals: map[string]string{}}
	for _, kv := range environ {
		// Windows has hidden entries such as "=C:=C:\Users", whose name starts with "=".
		if kv == "" {
			continue
		}
		if i := strings.IndexByte(kv[1:], '='); i >= 0 {
			e.set(kv[:i+1], kv[i+2:])
		}
	}
	return e
}

func (e *envSet) get(name string) (string, bool) {
	v, ok := e.vals[strings.ToUpper(name)]
	return v, ok
}

func (e *envSet) set(name, val string) {
	k := strings.ToUpper(name)
	if _, ok := e.vals[k]; !ok {
		e.names = append(e.names, name)
	}
	e.vals[k] = val
}

func (e *envSet) list() []string {
	out := make([]string, len(e.names))
	for i, n := range e.names {
		out[i] = n + "=" + e.vals[strings.ToUpper(n)]
	}
	return out
}

// expand replaces %NAME% with the variable's value, like Windows' ExpandEnvironmentStrings:
// an unknown name stays as written, and its closing % may open the next name.
func (e *envSet) expand(s string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '%')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i+1:]
		j := strings.IndexByte(s, '%')
		if j < 0 {
			b.WriteByte('%')
			b.WriteString(s)
			return b.String()
		}
		if v, ok := e.get(s[:j]); ok && j > 0 {
			b.WriteString(v)
			s = s[j+1:]
			continue
		}
		b.WriteByte('%')
	}
}

// lineWriter passes a script's output to the installer's details, one line at a time.
type lineWriter struct {
	log  func(string)
	part []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.part = append(w.part, p...)
	for {
		i := strings.IndexAny(string(w.part), "\r\n")
		if i < 0 {
			return len(p), nil
		}
		if line := strings.TrimSpace(string(w.part[:i])); line != "" {
			w.log(line)
		}
		w.part = w.part[i+1:]
	}
}

func (w *lineWriter) flush() {
	if line := strings.TrimSpace(string(w.part)); line != "" {
		w.log(line)
	}
	w.part = nil
}
