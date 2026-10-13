//go:build !windows

package main

import (
	"encoding/xml"
	"reflect"
	"strings"
	"testing"
)

func TestBrewPackages(t *testing.T) {
	for id, want := range map[string][]string{
		"cask:google-chrome":                     {"install", "--cask", "google-chrome"},
		"formula:node@22":                        {"install", "--formula", "node@22"},
		"formula:mongodb/brew/mongodb-community": {"install", "--formula", "mongodb/brew/mongodb-community"},
		"firefox":                                {"install", "firefox"},
	} {
		if !validBrew(id) {
			t.Errorf("%s should be valid", id)
		}
		if got := brewArgs(id); !reflect.DeepEqual(got, want) {
			t.Errorf("brewArgs(%s) = %v", id, got)
		}
	}
	for _, id := range []string{"", "Google.Chrome", "cask:", "--force", "cask:a b", "a/../b", "tap:x", "a/b/c/d"} {
		if validBrew(id) {
			t.Errorf("%q should be refused", id)
		}
	}
}

func TestMacAppsAreHomebrewPackages(t *testing.T) {
	macInstaller = true
	defer func() { macInstaller = false }()
	apps := cleanApps([]App{{ID: "cask:vlc", Name: "VLC", URL: "https://example.com/x.exe"}, {ID: "Google.Chrome"}, {ID: "node@22"}})
	if len(apps) != 2 || apps[0].ID != "cask:vlc" || apps[0].URL != "" || !apps[0].Fixed || apps[1].Name != "node@22" {
		t.Fatalf("got %+v", apps)
	}
}

func TestBrewUpdatesPlist(t *testing.T) {
	p := brewUpdatesPlist("/opt/homebrew/bin/brew")
	if err := xml.Unmarshal([]byte(p), new(struct{})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, "<string>/opt/homebrew/bin/brew</string><string>upgrade</string>") {
		t.Fatal(p)
	}
}

func TestLineWriter(t *testing.T) {
	var got []string
	w := &lineWriter{log: func(s string) { got = append(got, s) }}
	w.Write([]byte("one\r\ntw"))
	w.Write([]byte("o\n\nthree"))
	w.flush()
	if !reflect.DeepEqual(got, []string{"one", "two", "three"}) {
		t.Fatal(got)
	}
}
