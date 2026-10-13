package main

// The web installer agent: the Crescent page opens crescent://install?token=…, the system starts this
// program, and the page itself sends the app list and shows the progress by talking to it on
// 127.0.0.1:agentPort. Only the allowed page origins with the token from the link get an answer, so
// another website can't make Crescent install anything.

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sync"
	"time"
)

const agentPort = "47615"

var agentToken = regexp.MustCompile(`^[A-Za-z0-9]{16,64}$`)

// linkToken returns the token in a crescent://install?token=… link, or "" if it has none.
func linkToken(link string) string {
	u, err := url.Parse(link)
	if err != nil || !agentToken.MatchString(u.Query().Get("token")) {
		return ""
	}
	return u.Query().Get("token")
}

// agentVersion goes up with every change to the agent; the page (AGENT_VERSION in web/index.html)
// sends a newer program to an older agent, which replaces itself for the next Install.
const agentVersion = 6

// agentOrigins are the pages that may drive the agent.
// The local page (setup.cmd) and the hosted site on Cloudflare Pages.
var agentOrigins = map[string]bool{"http://localhost:8765": true, "http://127.0.0.1:8765": true, "https://crescent-installer.pages.dev": true}

// agentState is what GET /state returns; the page redraws its installing screen from it.
type agentState struct {
	Version  int      `json:"version"`
	Started  bool     `json:"started"`
	Done     int      `json:"done"`
	Total    int      `json:"total"`
	Status   string   `json:"status"`
	Outcomes []string `json:"outcomes"` // per app: "", installed, skipped, failed or cancelled
	Log      []string `json:"log"`
	Finished bool     `json:"finished"`
	Result   any      `json:"result,omitempty"`
}

type agent struct {
	token string
	in    *Installer
	// self is the installed agent program that POST /update replaces ("" turns updates off).
	self string
	// run does the install (apps, weekly updates, scripts) and returns the summary for the page.
	run func(a *agent, p *Payload, apps []App) any
	// started is closed when an install begins, finished when its result is in, and seen when the
	// page has read that result.
	started, finished, seen chan struct{}
	mu                      sync.Mutex
	st                      agentState
	once, seenOnce          sync.Once
}

func newAgent(token string, in *Installer, run func(*agent, *Payload, []App) any) *agent {
	return &agent{token: token, in: in, run: run, started: make(chan struct{}), finished: make(chan struct{}), seen: make(chan struct{}),
		st: agentState{Version: agentVersion, Outcomes: []string{}, Log: []string{}}}
}

func (a *agent) Progress(done, total int, status string) {
	a.mu.Lock()
	a.st.Done, a.st.Total, a.st.Status = done, total, status
	a.mu.Unlock()
}
func (a *agent) Outcome(i int, outcome string) {
	a.mu.Lock()
	if i < len(a.st.Outcomes) {
		a.st.Outcomes[i] = outcome
	}
	a.mu.Unlock()
}
func (a *agent) Log(line string) {
	a.mu.Lock()
	a.st.Log = append(a.st.Log, line)
	if len(a.st.Log) > 300 {
		a.st.Log = a.st.Log[len(a.st.Log)-300:]
	}
	a.mu.Unlock()
}

func (a *agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if !agentOrigins[origin] {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Vary", "Origin")
	if r.Method == http.MethodOptions {
		h.Set("Access-Control-Allow-Methods", "GET, POST")
		h.Set("Access-Control-Allow-Headers", "Content-Type, X-Crescent-Token")
		// Chrome asks before a public page reaches a program on this PC (Private Network Access).
		h.Set("Access-Control-Allow-Private-Network", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Crescent-Token")), []byte(a.token)) != 1 {
		http.Error(w, "wrong token", http.StatusForbidden)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/state":
		a.mu.Lock()
		b, _ := json.Marshal(a.st)
		done := a.st.Finished
		a.mu.Unlock()
		h.Set("Content-Type", "application/json")
		w.Write(b)
		if done {
			a.seenOnce.Do(func() { close(a.seen) })
		}
	case r.Method == http.MethodPost && r.URL.Path == "/start":
		var p Payload
		// Scripts can be up to 20 x 1,000,000 characters.
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<20)).Decode(&p); err != nil {
			http.Error(w, "bad app list", http.StatusBadRequest)
			return
		}
		payload, apps, err := cleanPayload(&p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		begun := false
		a.once.Do(func() {
			begun = true
			a.mu.Lock()
			a.st.Started, a.st.Total, a.st.Outcomes = true, len(apps), make([]string, len(apps))
			a.mu.Unlock()
			close(a.started)
			go func() {
				res := a.run(a, payload, apps)
				a.mu.Lock()
				a.st.Finished, a.st.Result = true, res
				a.mu.Unlock()
				close(a.finished)
			}()
		})
		if !begun {
			http.Error(w, "already installing", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && r.URL.Path == "/update":
		exe, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		if err == nil {
			err = a.update(exe)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && r.URL.Path == "/cancel":
		a.in.Cancel()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

// agentResult is the summary the page shows when the install ends.
func agentResult(r Result) any {
	return map[string]any{"installed": r.Installed, "skipped": r.Skipped, "failed": append([]string{}, r.Failed...),
		"notes": append([]string{}, r.Notes...), "cancelled": r.Cancelled}
}

// serve answers the page on 127.0.0.1:agentPort until it has seen the result of the install, then
// returns. It fails when another agent keeps the port.
func (a *agent) serve() error {
	// A previous install may still be closing; give it a few seconds to free the port.
	var ln net.Listener
	var err error
	for try := 0; try < 10; try++ {
		if ln, err = net.Listen("tcp", "127.0.0.1:"+agentPort); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	go http.Serve(ln, a)
	select {
	case <-a.started:
	case <-time.After(3 * time.Minute): // the page never sent a list
		return nil
	}
	<-a.finished
	// Quit as soon as the page has the result (it asks about once a second), or after 5 seconds if
	// the page was closed, so the next Install starts a fresh agent.
	select {
	case <-a.seen:
		time.Sleep(200 * time.Millisecond) // let that last answer reach the page
	case <-time.After(5 * time.Second):
	}
	return nil
}

// isProgram tells whether exe is a program for this system: a Windows .exe, or a Mac program
// (universal or single-architecture).
func isProgram(exe []byte) bool {
	if runtime.GOOS == "windows" {
		return bytes.HasPrefix(exe, []byte("MZ"))
	}
	return bytes.HasPrefix(exe, []byte{0xca, 0xfe, 0xba, 0xbe}) || bytes.HasPrefix(exe, []byte{0xcf, 0xfa, 0xed, 0xfe})
}

// update replaces the installed agent with exe, the plain program (no app list) the page sent. This
// run carries on; the next Install starts the new one. Windows lets a running program be renamed,
// not overwritten, so the old one moves aside to .old and is removed next time.
func (a *agent) update(exe []byte) error {
	if a.self == "" {
		return errors.New("updates are off")
	}
	if !isProgram(exe) {
		return errors.New("not a program for this computer")
	}
	if _, err := readPayload(exe); !errors.Is(err, errNoPayload) {
		return errors.New("not the Crescent agent")
	}
	tmp, old := a.self+".new", a.self+".old"
	if err := os.WriteFile(tmp, exe, 0o755); err != nil {
		return err
	}
	os.Remove(old)
	if err := os.Rename(a.self, old); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, a.self)
}
