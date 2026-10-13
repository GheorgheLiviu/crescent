package main

import (
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentGuardsAndRuns(t *testing.T) {
	ran := make(chan []App, 1)
	a := newAgent("tok1234567890abcd", &Installer{}, func(a *agent, p *Payload, apps []App) any {
		a.Outcome(0, "installed")
		ran <- apps
		return "ok"
	})
	do := func(method, path, origin, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if token != "" {
			r.Header.Set("X-Crescent-Token", token)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	const page, tok = "http://localhost:8765", "tok1234567890abcd"
	list := `{"apps":[{"id":"Git.Git","name":"Git"}]}`
	if w := do("POST", "/start", "https://evil.example", tok, list); w.Code != http.StatusForbidden {
		t.Fatalf("other site: %d", w.Code)
	}
	if w := do("POST", "/start", "", tok, list); w.Code != http.StatusForbidden {
		t.Fatalf("no origin: %d", w.Code)
	}
	if w := do("POST", "/start", page, "wrong", list); w.Code != http.StatusForbidden {
		t.Fatalf("wrong token: %d", w.Code)
	}
	if w := do("OPTIONS", "/start", page, "", ""); w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Private-Network") != "true" {
		t.Fatalf("preflight: %d %v", w.Code, w.Header())
	}
	if w := do("POST", "/start", page, tok, `{"apps":[{"id":"bad id"}]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("no valid apps: %d", w.Code)
	}
	if w := do("POST", "/start", page, tok, list); w.Code != http.StatusNoContent {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	if apps := <-ran; len(apps) != 1 || apps[0].ID != "Git.Git" {
		t.Fatalf("ran %v", apps)
	}
	<-a.finished
	if w := do("POST", "/start", page, tok, list); w.Code != http.StatusConflict {
		t.Fatalf("second start: %d", w.Code)
	}
	w := do("GET", "/state", page, tok, "")
	if body := w.Body.String(); !strings.Contains(body, `"finished":true`) || !strings.Contains(body, `"outcomes":["installed"]`) {
		t.Fatalf("state: %s", body)
	}
	select {
	case <-a.seen:
	default:
		t.Fatal("reading the result should let the agent quit")
	}
}

func TestAgentUpdate(t *testing.T) {
	self := filepath.Join(t.TempDir(), "Crescent.exe")
	os.WriteFile(self, []byte("MZold"), 0o755)
	a := &agent{self: self}
	list := `{"apps":[]}`
	withList := append([]byte("MZnew"+list), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(withList[len(withList)-4:], uint32(len(list)))
	withList = append(withList, magic...)
	for _, bad := range [][]byte{[]byte("not an exe"), withList} {
		if a.update(bad) == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if err := a.update([]byte("MZnew")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(self); string(b) != "MZnew" {
		t.Fatalf("installed %q", b)
	}
	if b, _ := os.ReadFile(self + ".old"); string(b) != "MZold" {
		t.Fatalf("old copy %q", b)
	}
}
