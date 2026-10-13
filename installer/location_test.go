//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidFolder(t *testing.T) {
	for p, want := range map[string]bool{
		`D:\Program Files\Steam`:         true,
		`d:\Apps\Notepad++`:              true,
		`E:\Jocuri\Ubisoft Connect`:      true,
		`D:\`:                            false, // apps always get their own folder
		`D:`:                             false,
		`D:Apps`:                         false,
		`\\nas\share\Apps`:               false,
		`D:\Apps\`:                       false, // a trailing \ would break winget's quoting
		`D:\Apps\"x`:                     false,
		`D:\Apps\a|b`:                    false,
		`D:\Apps\..\Windows`:             false,
		`D:\Apps\name.`:                  false,
		`D:\Apps\ name`:                  false,
		`D:\Apps\x` + "\n":               false,
		`D:\` + strings.Repeat("a", 200): false,
	} {
		if got := validFolder(p); got != want {
			t.Errorf("validFolder(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestCleanAppsMarksAppsThatCantMove(t *testing.T) {
	apps := cleanApps([]App{
		{ID: "Valve.Steam", Name: "Steam"},
		{ID: "Google.Chrome", Name: "Google Chrome", Fixed: true},
		{ID: "9NKSQGP7F2NH", Name: "WhatsApp"},
		{ID: "Microsoft.Office", Name: "Microsoft 365", Office: []string{"Word"}},
		{ID: "Samsung.Magician", Name: "Samsung Magician", URL: magicianURL},
	})
	got := map[string]bool{}
	for _, a := range apps {
		got[a.ID] = a.Fixed
	}
	want := map[string]bool{"Valve.Steam": false, "Google.Chrome": true, "9NKSQGP7F2NH": true, "Microsoft.Office": true, "Samsung.Magician": true}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s fixed = %v, want %v", id, got[id], w)
		}
	}
}

// The Office settings file exists while winget runs and is removed afterwards.
func TestOfficeConfigRemoved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	cfg := filepath.Join(dir, "crescent-office.xml")
	winget := filepath.Join(dir, "winget")
	if err := os.WriteFile(winget, []byte("#!/bin/sh\ntest -f "+cfg+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ui := &recUI{outcomes: map[int]string{}}
	r := (&Installer{Winget: winget}).Run(cleanApps([]App{{ID: "Microsoft.Office", Office: []string{"Word"}}}), ui)
	if r.Installed != 1 {
		t.Fatalf("winget didn't find the settings file: %+v %v", r, ui.log)
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Errorf("settings file left behind: %v", err)
	}
}

// TestRunPassesLocation runs the installer against a fake winget that records its arguments.
func TestRunPassesLocation(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	winget := filepath.Join(dir, "winget")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s|' \"$a\" >> " + argsFile + "; done\necho >> " + argsFile + "\n"
	if err := os.WriteFile(winget, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	apps := cleanApps([]App{
		{ID: "Valve.Steam", Name: "Steam"},
		{ID: "VideoLAN.VLC", Name: "VLC media player"},
		{ID: "Google.Chrome", Name: "Google Chrome", Fixed: true},
		{ID: "Git.Git", Name: "Git"},
	})
	setLocations(apps, []string{`E:\Games\Steam`, `D:\Program Files\VLC media player`, `D:\Program Files\Google Chrome`, `D:\Apps\`})
	ui := &recUI{outcomes: map[int]string{}}
	r := (&Installer{Winget: winget}).Run(apps, ui)
	if r.Installed != 4 {
		t.Fatalf("installed %d, want 4: %v", r.Installed, ui.log)
	}
	out, _ := os.ReadFile(argsFile)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	want := []string{`--location|E:\Games\Steam|`, `--location|D:\Program Files\VLC media player|`, "", ""}
	for i, w := range want {
		has := strings.Contains(lines[i], "--location")
		if w == "" && has || w != "" && !strings.Contains(lines[i], w) {
			t.Errorf("app %d: winget got %q, want location %q", i, lines[i], w)
		}
	}
	if !strings.Contains(strings.Join(ui.log, "\n"), `Folder: E:\Games\Steam`) {
		t.Errorf("log doesn't name the folder: %v", ui.log)
	}
}
