//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakePrograms puts shell scripts named after programs on the Path; each logs its arguments.
func fakePrograms(t *testing.T, progs map[string]string) {
	dir := t.TempDir()
	for name, body := range progs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho \""+name+" $*\"\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin")
}

func runAddonTest(id string) (*recUI, Result) {
	ui := &recUI{outcomes: map[int]string{}}
	var r Result
	new(Installer).runAddon(0, App{ID: id, Name: id}, ui, &r)
	return ui, r
}

func TestAddonsSetUp(t *testing.T) {
	// The marketplace is already there (it fails), and the plugin still installs.
	fakePrograms(t, map[string]string{"uv": "", "graphify": "", "claude": `[ "$2" = marketplace ] && exit 1; exit 0`})
	ui, r := runAddonTest("Crescent.Graphify")
	want := []string{"uv tool install --upgrade graphifyy", "uv tool update-shell", "graphify install"}
	if !reflect.DeepEqual(ui.log, want) || ui.outcomes[0] != "installed" || r.Installed != 1 {
		t.Fatalf("graphify: %v %v %+v", ui.log, ui.outcomes, r)
	}
	ui, r = runAddonTest("Crescent.Ponytail")
	want = []string{"claude plugin marketplace add DietrichGebert/ponytail", "claude plugin install ponytail@ponytail"}
	if !reflect.DeepEqual(ui.log, want) || ui.outcomes[0] != "installed" || r.Installed != 1 {
		t.Fatalf("ponytail: %v %v %+v", ui.log, ui.outcomes, r)
	}
}

func TestAddonFails(t *testing.T) {
	fakePrograms(t, map[string]string{"uv": `[ "$2" = install ] && exit 3; exit 0`})
	ui, r := runAddonTest("Crescent.Graphify")
	if ui.outcomes[0] != "failed" || len(r.Failed) != 1 || len(ui.log) != 2 {
		t.Fatalf("%v %v %+v", ui.log, ui.outcomes, r)
	}
	fakePrograms(t, nil) // no Claude Code
	ui, _ = runAddonTest("Crescent.Ponytail")
	if ui.outcomes[0] != "failed" || !strings.Contains(strings.Join(ui.log, "\n"), "claude wasn't found") {
		t.Fatalf("%v %v", ui.log, ui.outcomes)
	}
}

func TestAddonsGoAfterApps(t *testing.T) {
	list := []App{{ID: "Crescent.Ponytail", Name: "Ponytail", URL: "https://x.example/a.exe"}, {ID: "Samsung.Magician", URL: magicianURL},
		{ID: "Anthropic.ClaudeCode"}, {ID: "Crescent.Nothing"}}
	for _, mac := range []bool{false, true} {
		macInstaller = mac
		var ids []string
		for _, a := range cleanApps(list) {
			ids = append(ids, a.ID)
			if isAddon(a.ID) && (!a.Fixed || a.URL != "") {
				t.Errorf("add-on %+v", a)
			}
		}
		want := []string{"Anthropic.ClaudeCode", "Crescent.Nothing", "Crescent.Ponytail", "Samsung.Magician"}
		if mac {
			want = []string{"Crescent.Ponytail"} // the Mac only takes Homebrew packages and add-ons
		}
		if !reflect.DeepEqual(ids, want) {
			t.Errorf("mac %v: %v", mac, ids)
		}
	}
	macInstaller = false
}
