package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
)

//go:embed ui.html
var uiHTML string

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	procMessageBox = user32.NewProc("MessageBoxW")
	procGetDpi     = user32.NewProc("GetDpiForSystem")
	mbOK           = uintptr(0x00000000)
	mbYesNo        = uintptr(0x00000004)
	mbIconError    = uintptr(0x00000010)
	mbIconQuestion = uintptr(0x00000020)
	mbIconInfo     = uintptr(0x00000040)
	idYes          = uintptr(6)
)

func msgBox(text string, flags uintptr) uintptr {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString("Crescent")
	r, _, _ := procMessageBox.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), flags)
	return r
}

// scaled converts a size in 96-DPI pixels to the system DPI, so the window isn't tiny on high-DPI screens.
func scaled(px uint) uint {
	if procGetDpi.Find() != nil {
		return px
	}
	dpi, _, _ := procGetDpi.Call()
	if dpi == 0 {
		return px
	}
	return px * uint(dpi) / 96
}

func openStore() {
	cmd := exec.Command("cmd", "/c", "start", "", "ms-windows-store://pdp/?productid=9NBLGGH4NNS1")
	hideConsole(cmd)
	cmd.Run()
}

// webUI forwards installer progress to the page inside the window.
type webUI struct{ w webview2.WebView }

func (u webUI) call(fn string, args ...any) {
	parts := make([]string, len(args))
	for i, a := range args {
		b, _ := json.Marshal(a)
		parts[i] = string(b)
	}
	js := fmt.Sprintf("window.ui && window.ui.%s(%s)", fn, strings.Join(parts, ","))
	u.w.Dispatch(func() { u.w.Eval(js) })
}
func (u webUI) Progress(done, total int, status string) { u.call("progress", done, total, status) }
func (u webUI) Outcome(i int, outcome string)           { u.call("outcome", i, outcome) }
func (u webUI) Log(line string)                         { u.call("log", line) }

type silentUI struct{}

func (silentUI) Progress(int, int, string) {}
func (silentUI) Outcome(int, string)       {}
func (silentUI) Log(string)                {}

// finish turns on weekly updates if asked and collects the lines shown at the end.
func finish(winget string, payload *Payload, r *Result) {
	if payload.AutoUpdate && !r.Cancelled {
		if err := scheduleUpdates(winget); err != nil {
			r.Notes = append(r.Notes, "Couldn't turn on weekly updates: "+err.Error())
		} else {
			r.Notes = append(r.Notes, `Weekly updates are on. To turn them off, delete "`+updateTaskName+`" in Task Scheduler.`)
		}
	}
}

func main() {
	// The page's Install button opens crescent://…; with no app list, this program sets that up.
	if len(os.Args) > 1 && strings.HasPrefix(strings.ToLower(os.Args[1]), "crescent:") {
		runAgent(os.Args[1])
		return
	}
	payload, apps, err := loadApps()
	if errors.Is(err, errNoPayload) {
		registerAgent()
		return
	}
	if err != nil {
		msgBox(capitalize(err.Error())+".", mbOK|mbIconError)
		return
	}
	winget, err := exec.LookPath("winget")
	if err != nil {
		if msgBox("This PC doesn't have winget (App Installer) yet, which Crescent needs.\n\nOpen the Microsoft Store to install it? Then run Crescent again.", mbYesNo|mbIconQuestion) == idYes {
			openStore()
		}
		return
	}
	in := &Installer{Winget: winget}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(os.TempDir(), "Crescent-WebView2"),
		WindowOptions: webview2.WindowOptions{
			Title:  "Crescent",
			Width:  scaled(680),
			Height: scaled(640),
			IconId: 1,
			Center: true,
		},
	})
	if w == nil {
		// No WebView2 runtime: install anyway, with message boxes instead of a window.
		names := make([]string, len(apps))
		for i, a := range apps {
			names[i] = a.Name
		}
		if msgBox(fmt.Sprintf("Install these %d apps now?\n\n%s\n\nThis can take a while. A message appears when it's done.", len(apps), strings.Join(names, "\n")), mbYesNo|mbIconQuestion) != idYes {
			return
		}
		r := in.Run(apps, silentUI{})
		finish(winget, payload, &r)
		if scripts := payload.Scripts; len(scripts) > 0 && msgBox(scriptsQuestion(scripts), mbYesNo|mbIconQuestion) == idYes {
			for i := range scripts {
				note, _ := in.RunScript(&scripts[i], silentUI{})
				r.Notes = append(r.Notes, note)
			}
		}
		msgBox("Done.\n\n"+r.Summary()+"\n\n"+strings.Join(r.Notes, "\n"), mbOK|mbIconInfo)
		return
	}
	defer w.Destroy()
	w.SetSize(int(scaled(680)), int(scaled(640)), webview2.HintFixed)

	type view struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Icon string `json:"icon,omitempty"`
		// Manual apps open their own setup window for the user to finish.
		Manual bool `json:"manual,omitempty"`
		// Fixed apps can't be given a folder.
		Fixed bool `json:"fixed,omitempty"`
	}
	vs := make([]view, len(apps))
	for i, a := range apps {
		vs[i] = view{a.ID, a.Name, a.Icon, a.URL != "", a.Fixed}
	}
	data := map[string]any{"apps": vs, "autoUpdate": payload.AutoUpdate}
	scripts := payload.Scripts
	if len(scripts) > 0 {
		list := make([]map[string]any, len(scripts))
		for i, s := range scripts {
			list[i] = map[string]any{"name": s.Name, "kind": s.Kind, "body": s.Body, "lines": scriptLines(s.Body)}
		}
		data["scripts"] = list
	}
	init, _ := json.Marshal(data)
	w.Init("window.AI = " + string(init) + ";")

	ui := webUI{w}
	// started: Install was pressed. finished: the Done screen is up. rerun: the Done screen's
	// "Run scripts" button was pressed (it works once). ran[i]: scripts[i] has run.
	var started, finished, rerun atomic.Bool
	ran := make([]bool, len(scripts))
	// running counts the script in progress, waited for on exit so its temporary copy is removed.
	// Once the window has closed (closing, under closeMu), no other script starts.
	var (
		running sync.WaitGroup
		closeMu sync.Mutex
		closing bool
	)
	// runScript runs scripts[i] in its own window and tells the page; begun is false when the
	// window has closed, and then nothing ran.
	runScript := func(i int) (note string, begun bool) {
		closeMu.Lock()
		if closing {
			closeMu.Unlock()
			return "", false
		}
		running.Add(1)
		closeMu.Unlock()
		defer running.Done()
		ui.call("scriptStarted", i)
		note, ok := in.RunScript(&scripts[i], ui)
		ui.call("scriptFinished", map[string]any{"index": i, "note": note, "ok": ok})
		return note, true
	}
	// locations[i] is the folder chosen for apps[i] on the Ready screen, or "" for its usual place.
	// runScripts[i] is the Ready screen's checkbox for scripts[i]; missing entries are off.
	w.Bind("aiStart", func(locations []string, runScripts []bool) {
		if !started.CompareAndSwap(false, true) {
			return
		}
		setLocations(apps, locations)
		go func() {
			r := in.Run(apps, ui)
			finish(winget, payload, &r)
			// Cancel skips the scripts that haven't started; the Done screen can still run them.
			for i := range scripts {
				if i >= len(runScripts) || !runScripts[i] || in.cancelled.Load() {
					continue
				}
				note, begun := runScript(i)
				if !begun {
					break
				}
				r.Notes = append(r.Notes, note)
				ran[i] = true
			}
			failed := r.Failed
			if failed == nil {
				failed = []string{}
			}
			notes := r.Notes
			if notes == nil {
				notes = []string{}
			}
			scriptsRan := append([]bool{}, ran...) // a copy: aiRunScripts may run once finished is set
			finished.Store(true)
			ui.call("done", map[string]any{"installed": r.Installed, "skipped": r.Skipped, "failed": failed, "notes": notes,
				"cancelled": r.Cancelled, "scriptsRan": scriptsRan})
		}()
	})
	// The Done screen's "Run scripts" button: runs, in order, the scripts that didn't run during the
	// install (switched off or skipped by Cancel). The user asked, so Cancel doesn't stop them.
	w.Bind("aiRunScripts", func() {
		if !finished.Load() || !rerun.CompareAndSwap(false, true) {
			return
		}
		go func() {
			for i := range scripts {
				if ran[i] {
					continue
				}
				if _, begun := runScript(i); !begun {
					return
				}
				ran[i] = true
			}
			ui.call("scriptsDone")
		}()
	})
	// The folder picker runs its own message loop, so it opens after this call returns rather than
	// inside WebView2's message handler, then hands the chosen folder ("" if none) to the page.
	w.Bind("aiPickFolder", func(title, start string) {
		if started.Load() {
			return
		}
		w.Dispatch(func() {
			b, _ := json.Marshal(pickFolder(uintptr(w.Window()), title, start))
			w.Eval("window.ui && window.ui.picked(" + string(b) + ")")
		})
	})
	w.Bind("aiCancel", func() { in.Cancel() })
	w.Bind("aiClose", func() { w.Dispatch(func() { w.Terminate() }) })
	w.SetHtml(uiHTML)
	w.Run()
	// Closing the window mid-install stops the app that is installing, and no further script starts.
	in.Cancel()
	closeMu.Lock()
	closing = true
	closeMu.Unlock()
	// A script keeps its own window open; wait for it so its temporary copy is removed.
	running.Wait()
}

// scriptsQuestion asks, when there is no installer window, whether to run the scripts.
func scriptsQuestion(scripts []Script) string {
	if len(scripts) == 1 {
		return "All apps are done.\n\nRun your script " + scripts[0].Name + " now? It runs as administrator in its own window."
	}
	names := make([]string, len(scripts))
	for i, s := range scripts {
		names[i] = s.Name
	}
	return fmt.Sprintf("All apps are done.\n\nRun your %d scripts now? They run as administrator, one after another, each in its own window.\n\n%s",
		len(scripts), strings.Join(names, "\n"))
}
