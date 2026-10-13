package main

// The Mac installer: "Crescent.app", whose app list is Contents/Resources/apps.json (the page
// writes it; appending it to the program would break the Mac's code signature check). The same
// installer window (ui.html) opens as a browser tab served from this Mac only, and Homebrew
// installs the apps, after installing Homebrew itself if it's missing.

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed ui.html
var uiHTML string

// alert shows a message box, for problems before the installer tab can open.
func alert(msg string) {
	exec.Command("osascript", "-e", "on run argv", "-e",
		`display alert "Crescent" message (item 1 of argv) as critical`, "-e", "end run", msg).Run()
}

// loadMacApps reads the app list the page put inside the app.
func loadMacApps() (*Payload, []App, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't locate this installer: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "..", "Resources", "apps.json"))
	if err != nil {
		return nil, nil, errors.New("this installer has no app list. Build a new one from the Crescent page")
	}
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, nil, errors.New("the app list in this installer is damaged. Build a new one from the Crescent page")
	}
	return cleanPayload(&p)
}

func findBrew() string {
	for _, p := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// askpass asks for the Mac password in a dialog when Homebrew or a script uses sudo; an app
// opened from Finder has no terminal to type it in.
const askpass = `#!/bin/sh
exec /usr/bin/osascript -e 'text returned of (display dialog "Crescent needs your Mac password to continue installing." default answer "" with hidden answer with title "Crescent" with icon caution)'
`

// setupEnv puts Homebrew's folders on the PATH and makes sudo use the password dialog, for
// Homebrew and the scripts. It returns a function that removes the helper files.
func setupEnv() func() {
	dir, err := os.MkdirTemp("", "Crescent-")
	if err != nil {
		return func() {}
	}
	if os.WriteFile(filepath.Join(dir, "askpass"), []byte(askpass), 0o700) == nil &&
		os.WriteFile(filepath.Join(dir, "sudo"), []byte("#!/bin/sh\nexec /usr/bin/sudo -A \"$@\"\n"), 0o700) == nil {
		os.Setenv("SUDO_ASKPASS", filepath.Join(dir, "askpass"))
	}
	os.Setenv("PATH", dir+":/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:"+os.Getenv("PATH"))
	os.Setenv("NONINTERACTIVE", "1")
	os.Setenv("HOMEBREW_NO_ENV_HINTS", "1")
	return func() { os.RemoveAll(dir) }
}

const (
	brewRelease   = "https://api.github.com/repos/Homebrew/brew/releases/latest"
	brewDownloads = "https://github.com/Homebrew/brew/releases/download/"
)

// installBrew installs Homebrew from its official .pkg, after the Mac confirms the package is
// signed by a developer Apple knows. The Mac asks for the password once.
func installBrew(ui UI) (string, error) {
	ui.Log("--- Homebrew ---")
	ui.Log("Homebrew isn't on this Mac yet. Downloading it…")
	curl := func(args ...string) ([]byte, error) {
		return exec.Command("/usr/bin/curl", append([]string{"--fail", "--silent", "--show-error", "--location",
			"--proto", "=https", "--proto-redir", "=https", "--connect-timeout", "30", "--retry", "2"}, args...)...).Output()
	}
	out, err := curl(brewRelease)
	if err != nil {
		return "", fmt.Errorf("couldn't reach GitHub: %w", err)
	}
	var rel struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	link := ""
	if json.Unmarshal(out, &rel) == nil {
		for _, a := range rel.Assets {
			if strings.HasSuffix(a.Name, ".pkg") && strings.HasPrefix(a.URL, brewDownloads) {
				link = a.URL
				break
			}
		}
	}
	if link == "" {
		return "", errors.New("couldn't find Homebrew's installer on GitHub")
	}
	dir, err := os.MkdirTemp("", "Crescent-brew-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	pkg := filepath.Join(dir, "Homebrew.pkg")
	if _, err := curl("--output", pkg, link); err != nil {
		return "", fmt.Errorf("couldn't download Homebrew: %w", err)
	}
	if out, err := exec.Command("/usr/sbin/spctl", "--assess", "--type", "install", pkg).CombinedOutput(); err != nil {
		return "", fmt.Errorf("Homebrew's installer didn't pass the Mac's security check: %s", strings.TrimSpace(string(out)))
	}
	ui.Log("Your Mac asks for your password to install Homebrew.")
	if out, err := exec.Command("/usr/bin/osascript", "-e", "on run argv", "-e",
		`do shell script "/usr/sbin/installer -pkg " & quoted form of (item 1 of argv) & " -target /" with administrator privileges`,
		"-e", "end run", pkg).CombinedOutput(); err != nil {
		return "", fmt.Errorf("Homebrew's installer stopped: %s", strings.TrimSpace(string(out)))
	}
	brew := findBrew()
	if brew == "" {
		return "", errors.New("Homebrew's installer finished, but Homebrew isn't there")
	}
	ui.Log("Homebrew is installed.")
	return brew, nil
}

// hub sends the installer's progress to the open tab (Server-Sent Events) and tracks whether a
// tab is still connected.
type hub struct {
	mu      sync.Mutex
	clients map[chan string]bool
	gone    time.Time // when the last tab disconnected (or the installer started)
}

func (h *hub) call(fn string, args ...any) {
	msg, _ := json.Marshal(append([]any{fn}, args...))
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- string(msg):
		default: // a tab that stopped reading
		}
	}
}

func (h *hub) serve(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	c := make(chan string, 4096)
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		if len(h.clients) == 0 {
			h.gone = time.Now()
		}
		h.mu.Unlock()
	}()
	fl.Flush()
	for {
		select {
		case m := <-c:
			fmt.Fprintf(w, "data: %s\n\n", m)
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// abandoned is true when no tab has been connected for a while.
func (h *hub) abandoned() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients) == 0 && time.Since(h.gone) > 90*time.Second
}

type tabUI struct{ h *hub }

func (u tabUI) Progress(done, total int, status string) { u.h.call("progress", done, total, status) }
func (u tabUI) Outcome(i int, outcome string)           { u.h.call("outcome", i, outcome) }
func (u tabUI) Log(line string)                         { u.h.call("log", line) }

// bridge gives the page in the tab the functions the Windows window binds (aiStart and the
// rest) and passes the installer's calls to window.ui.
const bridge = `(() => {
  const call = (name, ...args) => fetch("call/" + name, { method: "POST", body: JSON.stringify(args) }).catch(() => {});
  window.aiStart = (locations, runScripts) => call("start", locations, runScripts);
  window.aiCancel = () => call("cancel");
  window.aiRunScripts = () => call("runScripts");
  window.aiClose = () => {
    call("close");
    document.querySelector("main").innerHTML = '<div><h1>Crescent has closed</h1><p class="lede">You can close this tab.</p></div>';
    document.querySelector("footer").hidden = true;
  };
  const events = new EventSource("events");
  events.onmessage = e => { const [fn, ...args] = JSON.parse(e.data); window.ui && window.ui[fn] && window.ui[fn](...args); };
})();`

func main() {
	payload, apps, err := loadMacApps()
	if err != nil {
		alert(capitalize(err.Error()) + ".")
		return
	}
	defer setupEnv()()
	in := &Installer{Brew: findBrew()}

	type view struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Icon  string `json:"icon,omitempty"`
		Fixed bool   `json:"fixed"`
	}
	vs := make([]view, len(apps))
	for i, a := range apps {
		vs[i] = view{a.ID, a.Name, a.Icon, true}
	}
	data := map[string]any{"apps": vs, "autoUpdate": payload.AutoUpdate, "mac": true}
	scripts := payload.Scripts
	if len(scripts) > 0 {
		list := make([]map[string]any, len(scripts))
		for i, s := range scripts {
			list[i] = map[string]any{"name": s.Name, "kind": s.Kind, "body": s.Body, "lines": scriptLines(s.Body)}
		}
		data["scripts"] = list
	}
	init, _ := json.Marshal(data) // escapes <, > and &, so it can't end the script tag
	page := strings.Replace(uiHTML, "</head>", "<script>window.AI = "+string(init)+";\n"+bridge+"</script>\n</head>", 1)

	h := &hub{clients: map[chan string]bool{}, gone: time.Now()}
	ui := tabUI{h}
	quit := make(chan struct{})
	var quitOnce sync.Once
	stop := func() { quitOnce.Do(func() { close(quit) }) }
	// work counts what is running (the install, a script), waited for before the installer quits.
	var work sync.WaitGroup
	var busy atomic.Int32
	goWork := func(f func()) {
		work.Add(1)
		busy.Add(1)
		go func() {
			defer work.Done()
			defer busy.Add(-1)
			f()
		}()
	}

	var started, finished, rerun atomic.Bool
	ran := make([]bool, len(scripts))
	runScript := func(i int) string {
		ui.h.call("scriptStarted", i)
		note, ok := in.RunScript(&scripts[i], ui)
		ui.h.call("scriptFinished", map[string]any{"index": i, "note": note, "ok": ok})
		ran[i] = true
		return note
	}
	start := func(runScripts []bool) {
		if !started.CompareAndSwap(false, true) {
			return
		}
		goWork(func() {
			var r Result
			if in.Brew == "" {
				ui.Progress(0, len(apps), "Installing Homebrew")
				if brew, err := installBrew(ui); err != nil {
					ui.Log("Couldn't install Homebrew: " + err.Error())
					for i, a := range apps {
						ui.Outcome(i, "failed")
						r.Failed = append(r.Failed, a.Name)
					}
					r.Notes = append(r.Notes, "Homebrew couldn't be installed, so no app was: "+err.Error())
				} else {
					in.Brew = brew
				}
			}
			if in.Brew != "" {
				r = in.Run(apps, ui)
				if payload.AutoUpdate && !r.Cancelled {
					if path, err := scheduleBrewUpdates(in.Brew); err != nil {
						r.Notes = append(r.Notes, "Couldn't turn on weekly updates: "+err.Error())
					} else {
						r.Notes = append(r.Notes, "Weekly updates are on. To turn them off, delete "+path+".")
					}
				}
			}
			for i := range scripts {
				if i < len(runScripts) && runScripts[i] && !in.cancelled.Load() {
					r.Notes = append(r.Notes, runScript(i))
				}
			}
			failed, notes := r.Failed, r.Notes
			if failed == nil {
				failed = []string{}
			}
			if notes == nil {
				notes = []string{}
			}
			finished.Store(true)
			ui.h.call("done", map[string]any{"installed": r.Installed, "skipped": r.Skipped, "failed": failed, "notes": notes,
				"cancelled": r.Cancelled, "scriptsRan": append([]bool{}, ran...)})
		})
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		alert("Crescent couldn't start: " + err.Error())
		return
	}
	addr := ln.Addr().String()
	key := make([]byte, 16)
	rand.Read(key)
	// The secret path keeps other websites and programs from driving the installer.
	base := "/" + hex.EncodeToString(key) + "/"
	mux := http.NewServeMux()
	mux.HandleFunc(base, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != base {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, page)
	})
	mux.HandleFunc(base+"events", h.serve)
	mux.HandleFunc(base+"call/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var args []json.RawMessage
		json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&args)
		switch strings.TrimPrefix(r.URL.Path, base+"call/") {
		case "start":
			var runScripts []bool
			if len(args) > 1 {
				json.Unmarshal(args[1], &runScripts)
			}
			start(runScripts)
		case "cancel":
			in.Cancel()
		case "runScripts":
			// The last screen's "Run scripts": the ones that didn't run yet, once.
			if finished.Load() && rerun.CompareAndSwap(false, true) {
				goWork(func() {
					for i := range scripts {
						if !ran[i] {
							runScript(i)
						}
					}
					ui.h.call("scriptsDone")
				})
			}
		case "close":
			stop()
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != addr { // a web page reaching this port under another name
			http.Error(w, "wrong host", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})}
	go srv.Serve(ln)
	if err := exec.Command("/usr/bin/open", "http://"+addr+base).Run(); err != nil {
		alert("Crescent couldn't open your browser: " + err.Error())
		return
	}
	// Closing the tab doesn't stop an install; once nothing runs and no tab is open, quit.
	go func() {
		for range time.Tick(5 * time.Second) {
			if busy.Load() == 0 && h.abandoned() {
				stop()
			}
		}
	}()
	<-quit
	in.Cancel()
	work.Wait()
}
