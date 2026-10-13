//go:build !windows

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanScript(t *testing.T) {
	body := "Write-Host hi"
	cases := []struct {
		name string
		in   *Script
		want *Script // nil: dropped
	}{
		{"none", nil, nil},
		{"powershell", &Script{"setup.ps1", "ps1", body}, &Script{"setup.ps1", "ps1", body}},
		{"batch .cmd", &Script{"Setup PC (2).cmd", "cmd", "echo hi"}, &Script{"Setup PC (2).cmd", "cmd", "echo hi"}},
		{"batch .bat any case", &Script{"setup.BAT", "cmd", "echo hi"}, &Script{"setup.BAT", "cmd", "echo hi"}},
		{"ps1 any case", &Script{"Setup.PS1", "ps1", body}, &Script{"Setup.PS1", "ps1", body}},
		{"unknown kind", &Script{"setup.sh", "sh", "echo hi"}, nil},
		{"kind is case-sensitive", &Script{"setup.ps1", "PS1", body}, nil},
		{"no kind", &Script{"setup.ps1", "", body}, nil},
		{"extension of the other kind", &Script{"setup.cmd", "ps1", body}, &Script{"script.ps1", "ps1", body}},
		{"bat for powershell", &Script{"setup.bat", "ps1", body}, &Script{"script.ps1", "ps1", body}},
		{"no extension", &Script{"setup", "cmd", "echo hi"}, &Script{"script.cmd", "cmd", "echo hi"}},
		{"path", &Script{`..\..\Windows\setup.cmd`, "cmd", "echo hi"}, &Script{"script.cmd", "cmd", "echo hi"}},
		{"slash path", &Script{"a/setup.cmd", "cmd", "echo hi"}, &Script{"script.cmd", "cmd", "echo hi"}},
		{"starts with a dot", &Script{".setup.ps1", "ps1", body}, &Script{"script.ps1", "ps1", body}},
		{"quote", &Script{`a"b.ps1`, "ps1", body}, &Script{"script.ps1", "ps1", body}},
		{"percent", &Script{"100%.cmd", "cmd", "echo hi"}, &Script{"script.cmd", "cmd", "echo hi"}},
		{"Romanian letters", &Script{"instalare-ș.ps1", "ps1", body}, &Script{"script.ps1", "ps1", body}},
		{"60 characters", &Script{strings.Repeat("a", 56) + ".ps1", "ps1", body}, &Script{strings.Repeat("a", 56) + ".ps1", "ps1", body}},
		{"61 characters", &Script{strings.Repeat("a", 57) + ".ps1", "ps1", body}, &Script{"script.ps1", "ps1", body}},
		{"device name", &Script{"CON.cmd", "cmd", "echo hi"}, &Script{"script.cmd", "cmd", "echo hi"}},
		{"device name with more dots", &Script{"nul.setup.ps1", "ps1", body}, &Script{"script.ps1", "ps1", body}},
		{"not a device name", &Script{"console.cmd", "cmd", "echo hi"}, &Script{"console.cmd", "cmd", "echo hi"}},
		{"byte order mark removed", &Script{"a.ps1", "ps1", "\uFEFF" + body}, &Script{"a.ps1", "ps1", body}},
		{"only one byte order mark removed", &Script{"a.ps1", "ps1", "\uFEFF\uFEFF" + body}, &Script{"a.ps1", "ps1", "\uFEFF" + body}},
		{"empty", &Script{"a.ps1", "ps1", ""}, nil},
		{"only spaces and lines", &Script{"a.ps1", "ps1", " \r\n\t\n "}, nil},
		{"only a byte order mark", &Script{"a.ps1", "ps1", "\uFEFF\n"}, nil},
		{"NUL character", &Script{"a.cmd", "cmd", "echo a\x00b"}, nil},
		{"largest", &Script{"a.cmd", "cmd", strings.Repeat("x", maxScriptBytes)}, &Script{"a.cmd", "cmd", strings.Repeat("x", maxScriptBytes)}},
		{"too large", &Script{"a.cmd", "cmd", strings.Repeat("x", maxScriptBytes+1)}, nil},
		{"too large in UTF-8 bytes", &Script{"a.cmd", "cmd", strings.Repeat("ș", maxScriptBytes/2+1)}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cleanScript(c.in)
			if (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
				t.Fatalf("cleanScript = %+v, want %+v", got, c.want)
			}
			if got != nil && got == c.in {
				t.Errorf("cleanScript returned its argument instead of a copy")
			}
		})
	}
}

func TestScriptFile(t *testing.T) {
	ps := scriptFile(&Script{"a.ps1", "ps1", "Write-Host 'Bună ziua, ță'\nexit 0\r\n"})
	if want := "\xEF\xBB\xBFWrite-Host 'Bună ziua, ță'\r\nexit 0\r\n"; string(ps) != want {
		t.Errorf("ps1 file = %q, want %q", ps, want)
	}
	bat := scriptFile(&Script{"a.cmd", "cmd", "echo ș\r\necho b\n\necho c\rx"})
	if want := "@chcp 65001 >nul\r\necho ș\r\necho b\r\n\r\necho c\rx"; string(bat) != want {
		t.Errorf("cmd file = %q, want %q", bat, want)
	}
	if strings.HasPrefix(string(bat), "\xEF\xBB\xBF") {
		t.Errorf("cmd file starts with a byte order mark")
	}
}

func TestScriptLines(t *testing.T) {
	for body, want := range map[string]int{"a": 1, "a\n": 1, "a\r\nb": 2, "a\nb\n": 2, "a\n\nb\r\n\r\n": 3} {
		if got := scriptLines(body); got != want {
			t.Errorf("scriptLines(%q) = %d, want %d", body, got, want)
		}
	}
}

// shScripts runs scripts with sh in tests: it drops what scriptFile adds for Windows (the
// byte order mark or the chcp line, and the carriage returns), and records each script file.
func shScripts(t *testing.T) *[]string {
	var files []string
	old := scriptCommand
	scriptCommand = func(file, kind string) *exec.Cmd {
		files = append(files, file)
		head := "tail -n +2" // the chcp line
		if kind == "ps1" {
			head = "tail -c +4" // the byte order mark
		}
		return exec.Command("sh", "-c", `cp "$1" "$HOME/copy" && `+head+` "$1" | tr -d '\r' > "$1.sh" && exec sh "$1.sh"`, "sh", file)
	}
	t.Cleanup(func() { scriptCommand = old })
	return &files
}

func TestRunScript(t *testing.T) {
	for _, kind := range []string{"cmd", "ps1"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("AI_TEST_GREETING", "salut")
			files := shScripts(t)
			s := cleanScript(&Script{"My setup." + kind, kind, "pwd > where\necho \"$AI_TEST_GREETING\" > env\n"})
			ui := &recUI{outcomes: map[int]string{}}
			in := &Installer{}
			note, ok := in.RunScript(s, ui)
			if !ok || note != "Your script My setup."+kind+" finished." {
				t.Fatalf("RunScript = %q, %v; log: %v", note, ok, ui.log)
			}
			if where, _ := os.ReadFile(filepath.Join(home, "where")); strings.TrimSpace(string(where)) != home {
				t.Errorf("ran in %q, want the home folder %q", where, home)
			}
			if env, _ := os.ReadFile(filepath.Join(home, "env")); strings.TrimSpace(string(env)) != "salut" {
				t.Errorf("script environment lacks AI_TEST_GREETING: %q", env)
			}
			if copy, _ := os.ReadFile(filepath.Join(home, "copy")); string(copy) != string(scriptFile(s)) {
				t.Errorf("file on disk = %q, want %q", copy, scriptFile(s))
			}
			if len(*files) != 1 {
				t.Fatalf("scriptCommand called %d times", len(*files))
			}
			file := (*files)[0]
			if filepath.Base(file) != s.Name || !strings.HasPrefix(filepath.Base(filepath.Dir(file)), "Crescent-script-") {
				t.Errorf("script written to %q", file)
			}
			if _, err := os.Stat(filepath.Dir(file)); !os.IsNotExist(err) {
				t.Errorf("temporary folder left behind: %v", err)
			}
			log := strings.Join(ui.log, "\n")
			if !strings.HasPrefix(log, "--- Your script: My setup."+kind+" ---\n") || !strings.Contains(log, "Its window is open. Crescent continues when it closes.") {
				t.Errorf("log = %v", ui.log)
			}
			if in.current != nil || in.manual {
				t.Errorf("installer still tracks the script: %v, %v", in.current, in.manual)
			}
		})
	}
}

func TestRunScriptExitCode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	files := shScripts(t)
	ui := &recUI{outcomes: map[int]string{}}
	note, ok := (&Installer{}).RunScript(cleanScript(&Script{"fix.cmd", "cmd", "echo trying\nexit 3"}), ui)
	if ok || note != "Your script fix.cmd ended with exit code 3." {
		t.Fatalf("RunScript = %q, %v", note, ok)
	}
	if _, err := os.Stat(filepath.Dir((*files)[0])); !os.IsNotExist(err) {
		t.Errorf("temporary folder left behind: %v", err)
	}
	if ui.log[len(ui.log)-1] != note {
		t.Errorf("log doesn't end with the note: %v", ui.log)
	}
}

func TestRunScriptCantStart(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-shell")
	old := scriptCommand
	scriptCommand = func(file, kind string) *exec.Cmd { return exec.Command(missing, file) }
	t.Cleanup(func() { scriptCommand = old })
	ui := &recUI{outcomes: map[int]string{}}
	in := &Installer{}
	note, ok := in.RunScript(cleanScript(&Script{"a.ps1", "ps1", "x"}), ui)
	if ok || !strings.HasPrefix(note, "Couldn't run your script a.ps1: ") || !strings.Contains(note, "no-such-shell") {
		t.Fatalf("RunScript = %q, %v", note, ok)
	}
	if in.current != nil || in.manual {
		t.Errorf("installer tracks a script that never started")
	}
	if note2, ok2 := in.RunScript(nil, ui); note2 != "" || ok2 {
		t.Errorf("RunScript(nil) = %q, %v", note2, ok2)
	}
}

// The script's window is the user's: Cancel must neither kill it nor stop it from starting.
func TestCancelDoesntStopScript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	shScripts(t)
	in := &Installer{}
	in.Cancel() // pressed during the install; the caller still chose to run the script
	go func() {
		for {
			in.mu.Lock()
			running := in.current != nil && in.manual
			in.mu.Unlock()
			if running {
				in.Cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	ui := &recUI{outcomes: map[int]string{}}
	note, ok := in.RunScript(cleanScript(&Script{"slow.cmd", "cmd", "sleep 1\necho yes > done\n"}), ui)
	if !ok || note != "Your script slow.cmd finished." {
		t.Fatalf("RunScript = %q, %v; log: %v", note, ok, ui.log)
	}
	if b, _ := os.ReadFile(filepath.Join(home, "done")); strings.TrimSpace(string(b)) != "yes" {
		t.Errorf("script was stopped before its end")
	}
}

func TestCleanScripts(t *testing.T) {
	if got := cleanScripts(nil); got != nil {
		t.Errorf("cleanScripts(nil) = %v", got)
	}
	if got := cleanScripts([]Script{{"a.py", "py", "x"}, {"b.cmd", "cmd", " "}}); got != nil {
		t.Errorf("cleanScripts with only invalid scripts = %v, want nil", got)
	}
	in := []Script{
		{"dark-mode.ps1", "ps1", "Set-ItemProperty x"},
		{"bad.sh", "sh", "echo mac"},                   // not for Windows: dropped
		{"empty.cmd", "cmd", "\r\n"},                   // empty: dropped
		{`..\setup.bat`, "cmd", "echo hi"},             // bad name: renamed
		{"show-file-extensions.ps1", "ps1", "\uFEFFx"}, // byte order mark removed
		{"setup.ps1", "ps1", "a\x00b"},                 // NUL: dropped
		{"dark-mode.ps1", "ps1", "again"},              // same name twice: kept, each runs in its own folder
	}
	want := []Script{
		{"dark-mode.ps1", "ps1", "Set-ItemProperty x"},
		{"script.cmd", "cmd", "echo hi"},
		{"show-file-extensions.ps1", "ps1", "x"},
		{"dark-mode.ps1", "ps1", "again"},
	}
	got := cleanScripts(in)
	if len(got) != len(want) {
		t.Fatalf("cleanScripts = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("script %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if in[4].Body != "\uFEFFx" {
		t.Errorf("cleanScripts changed the list it was given")
	}

	// At most 20 are kept: the first 20 valid ones, in order.
	var many []Script
	for i := range 25 {
		many = append(many, Script{fmt.Sprintf("s%02d.cmd", i), "cmd", fmt.Sprint("echo ", i)})
		if i == 3 {
			many = append(many, Script{"bad.cmd", "cmd", ""}) // invalid ones don't count
		}
	}
	got = cleanScripts(many)
	if len(got) != maxScripts || maxScripts != 20 {
		t.Fatalf("kept %d scripts, want 20", len(got))
	}
	for i, s := range got {
		if s.Name != fmt.Sprintf("s%02d.cmd", i) {
			t.Errorf("script %d is %s", i, s.Name)
		}
	}
}

func exeWith(payload string) []byte {
	b := append([]byte("MZ...the installer program..."), payload...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(payload)))
	return append(b, magic...)
}

func TestParsePayloadScripts(t *testing.T) {
	apps := `"apps":[{"id":"Git.Git","name":"Git"}]`
	cases := []struct {
		name, json string
		want       []Script
	}{
		{"none", `{` + apps + `,"autoUpdate":true}`, nil},
		{"null", `{` + apps + `,"scripts":null}`, nil},
		{"empty list", `{` + apps + `,"scripts":[]}`, nil},
		{"one", `{` + apps + `,"autoUpdate":false,"scripts":[{"name":"setup.ps1","kind":"ps1","body":"\ufeffgit --version\r\nWrite-Host 'Gata, ș ț'"}]}`,
			[]Script{{"setup.ps1", "ps1", "git --version\r\nWrite-Host 'Gata, ș ț'"}}},
		{"in the page's order", `{` + apps + `,"scripts":[` +
			`{"name":"show-file-extensions.ps1","kind":"ps1","body":"a"},` +
			`{"name":"dark-mode.ps1","kind":"ps1","body":"b"},` +
			`{"name":"mine.bat","kind":"cmd","body":"c"},` +
			`{"name":"C:\\x\\setup.bat","kind":"cmd","body":"d"}]}`,
			[]Script{{"show-file-extensions.ps1", "ps1", "a"}, {"dark-mode.ps1", "ps1", "b"}, {"mine.bat", "cmd", "c"}, {"script.cmd", "cmd", "d"}}},
		{"invalid ones dropped", `{` + apps + `,"scripts":[{"name":"a.py","kind":"py","body":"print(1)"},{"name":"a.cmd","kind":"cmd","body":"  "},{"name":"b.cmd","kind":"cmd","body":"echo b"}]}`,
			[]Script{{"b.cmd", "cmd", "echo b"}}},
		{"the old single script is ignored", `{` + apps + `,"script":{"name":"a.cmd","kind":"cmd","body":"echo"}}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, got, err := parsePayload(exeWith(c.json))
			if err != nil || len(got) != 1 || got[0].ID != "Git.Git" {
				t.Fatalf("parsePayload = %v, %v", got, err)
			}
			if len(p.Scripts) != len(c.want) || (p.Scripts == nil) != (c.want == nil) {
				t.Fatalf("scripts = %+v, want %+v", p.Scripts, c.want)
			}
			for i := range c.want {
				if p.Scripts[i] != c.want[i] {
					t.Errorf("script %d = %+v, want %+v", i, p.Scripts[i], c.want[i])
				}
			}
		})
	}
	if _, _, err := parsePayload(exeWith(`{"apps":[],"scripts":[{"name":"a.cmd","kind":"cmd","body":"echo"}]}`)); err == nil {
		t.Errorf("scripts without apps were accepted")
	}
	if _, _, err := parsePayload(exeWith(`{"apps":[{"id":"Git.Git"}],"scripts":{"name":"a.cmd"}}`)); err == nil {
		t.Errorf("scripts that aren't a list were accepted")
	}
}

// Several scripts run one after another, in order, each from its own temporary folder.
func TestRunScriptsInOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	files := shScripts(t)
	scripts := cleanScripts([]Script{
		{"same.cmd", "cmd", "echo one >> order\n"},
		{"same.cmd", "cmd", "sleep 0.2; echo two >> order\n"},
		{"last.ps1", "ps1", "echo three >> order\n"},
	})
	ui := &recUI{outcomes: map[int]string{}}
	in := &Installer{}
	var notes []string
	for i := range scripts {
		note, ok := in.RunScript(&scripts[i], ui)
		if !ok {
			t.Fatalf("script %d: %q", i, note)
		}
		notes = append(notes, note)
	}
	if b, _ := os.ReadFile(filepath.Join(home, "order")); string(b) != "one\ntwo\nthree\n" {
		t.Errorf("ran in this order: %q", b)
	}
	if want := []string{"Your script same.cmd finished.", "Your script same.cmd finished.", "Your script last.ps1 finished."}; strings.Join(notes, "|") != strings.Join(want, "|") {
		t.Errorf("notes = %q", notes)
	}
	if len(*files) != 3 || filepath.Dir((*files)[0]) == filepath.Dir((*files)[1]) {
		t.Errorf("scripts written to %v", *files)
	}
}

func TestMergeEnv(t *testing.T) {
	base := []string{"=C:=C:\\Users\\User", "Path=C:\\old", "USERNAME=User", "USERPROFILE=C:\\Users\\User",
		"LOCALAPPDATA=C:\\Users\\User\\AppData\\Local", "SystemRoot=C:\\Windows", "TEMP=C:\\Users\\USER~1\\AppData\\Local\\Temp", "Keep=me"}
	machine := []envVar{
		{"Path", `%SystemRoot%\system32;;C:\Program Files\Git\cmd;%NVM_SYMLINK%;`, true},
		{"USERNAME", "SYSTEM", false},
		{"TEMP", `%SystemRoot%\TEMP`, true},
		{"NVM_SYMLINK", `C:\nvm4w\nodejs`, false},
		{"JAVA_HOME", `C:\Java`, false},
	}
	user := []envVar{
		{"PATH", `%USERPROFILE%\AppData\Local\Programs\Python\Python313\;%NVM_HOME%`, true},
		{"NVM_HOME", `%LOCALAPPDATA%\nvm`, true},
		{"TEMP", `%USERPROFILE%\AppData\Local\Temp`, true},
		{"temp_dir", `%TEMP%\x;%NOT_SET%;100%`, true},
	}
	got := map[string]string{}
	var names []string
	for _, kv := range mergeEnv(base, machine, user) {
		i := strings.IndexByte(kv[1:], '=') + 1
		names = append(names, kv[:i])
		got[kv[:i]] = kv[i+1:]
	}
	want := map[string]string{
		"=C:":         `C:\Users\User`,
		"Path":        `C:\Windows\system32;C:\Program Files\Git\cmd;C:\nvm4w\nodejs;C:\Users\User\AppData\Local\Programs\Python\Python313\;C:\Users\User\AppData\Local\nvm`,
		"USERNAME":    "User",
		"TEMP":        `C:\Users\User\AppData\Local\Temp`,
		"Keep":        "me",
		"NVM_SYMLINK": `C:\nvm4w\nodejs`,
		"JAVA_HOME":   `C:\Java`,
		"NVM_HOME":    `C:\Users\User\AppData\Local\nvm`,
		"temp_dir":    `C:\Users\User\AppData\Local\Temp\x;%NOT_SET%;100%`,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %q, want %q", k, got[k], w)
		}
	}
	if len(names) != len(want)+3 { // + USERPROFILE, LOCALAPPDATA, SystemRoot
		t.Errorf("names = %v", names)
	}
	if names[0] != "=C:" || names[1] != "Path" {
		t.Errorf("order or spelling changed: %v", names)
	}

	// Without the machine key, the Path is left as it was.
	if env := mergeEnv([]string{"Path=C:\\old"}, nil, []envVar{{"Path", `C:\user`, false}}); env[0] != `Path=C:\old` {
		t.Errorf("Path without machine key = %v", env)
	}
}

func TestEnvExpand(t *testing.T) {
	e := newEnvSet([]string{"A=1", "b=two"})
	for in, want := range map[string]string{
		"%A%":         "1",
		"%a%-%B%":     "1-two",
		"x%%y":        "x%%y",
		"%X%A%":       "%X1",
		"50% off %A%": "50% off 1", // " off " is no name, so its closing % opens the next one
		"%A":          "%A",
		"%":           "%",
		"no vars":     "no vars",
	} {
		if got := e.expand(in); got != want {
			t.Errorf("expand(%q) = %q, want %q", in, got, want)
		}
	}
}
