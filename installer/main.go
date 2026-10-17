// Crescent for Windows and Mac.
//
// One generic program: on Windows the web page appends the user's app list to the end of
// this .exe, and at run time the program reads it back and installs each app
// with winget. Trailer layout: <JSON payload><uint32 LE payload length><"AUTOINS1">.
// On the Mac the list is a file inside the app (main_darwin.go) and Homebrew installs the apps (brew.go).
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf16"
)

const magic = "AUTOINS1"

// winget exit codes that mean "nothing to do, it's already there".
const (
	errAlreadyInstalled = 0x8A150061
	errUpdateNotApplied = 0x8A15002B
)

type App struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Icon is an optional small image (data: URL) the page passes along for the installer window.
	Icon string `json:"icon,omitempty"`
	// Office lists the Microsoft Office apps to keep (Word, Excel, PowerPoint, Outlook, OneNote, Access)
	// when ID is Microsoft.Office; every other Office app is left out.
	Office []string `json:"office,omitempty"`
	// URL is set for apps that winget can't install silently: the vendor's own setup is
	// downloaded from it and opened for the user (see direct.go). Detect is the name the app
	// shows in Installed apps, used to skip it when it's already there.
	URL    string `json:"url,omitempty"`
	Detect string `json:"detect,omitempty"`
	// Fixed apps always install to their usual place: their setup ignores a chosen folder.
	Fixed bool `json:"fixed,omitempty"`
	// Location is the folder the user chose in the installer window, or "" for the usual place.
	Location string `json:"-"`
}

// Office apps the page can pick, plus the ones always left out.
var (
	officePickable  = []string{"Word", "Excel", "PowerPoint", "Outlook", "OneNote", "Access"}
	officeAlwaysOff = []string{"Groove", "Lync", "OutlookForWindows", "Publisher", "Teams", "Bing"}
)

// extraArgs adds winget options for packages that need more than a plain silent install.
var extraArgs = map[string][]string{
	// The Build Tools installer installs nothing useful unless a workload is named.
	"Microsoft.VisualStudio.2022.BuildTools": {"--override",
		"--quiet --wait --norestart --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"},
}

// randomPassword returns a 20-character password for the PostgreSQL superuser.
func randomPassword() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// savePostgresPassword writes the connection details to a file on the Desktop
// and returns a line telling the user where it is.
func savePostgresPassword(name, password string) string {
	text := fmt.Sprintf("%s\r\n\r\nHost: localhost\r\nPort: 5432\r\nUser: postgres\r\nPassword: %s\r\n", name, password)
	home, _ := os.UserHomeDir()
	for _, dir := range []string{filepath.Join(home, "OneDrive", "Desktop"), filepath.Join(home, "Desktop"), home} {
		path := filepath.Join(dir, "PostgreSQL password.txt")
		if os.WriteFile(path, []byte(text), 0o600) == nil {
			return "PostgreSQL login saved to " + path
		}
	}
	return "PostgreSQL login: user postgres, password " + password + " (write it down, it couldn't be saved to a file)"
}

// officeConfig writes an Office Deployment Tool configuration that installs
// only the chosen apps, and returns its path.
func officeConfig(keep []string) (string, error) {
	want := map[string]bool{}
	for _, k := range keep {
		want[k] = true
	}
	var b strings.Builder
	b.WriteString("<Configuration>\n  <Add OfficeClientEdition=\"64\" Channel=\"Current\">\n    <Product ID=\"O365HomePremRetail\">\n      <Language ID=\"MatchOS\" />\n")
	for _, app := range officePickable {
		if !want[app] {
			fmt.Fprintf(&b, "      <ExcludeApp ID=\"%s\" />\n", app)
		}
	}
	for _, app := range officeAlwaysOff {
		fmt.Fprintf(&b, "      <ExcludeApp ID=\"%s\" />\n", app)
	}
	b.WriteString("    </Product>\n  </Add>\n  <Display Level=\"None\" AcceptEULA=\"TRUE\" />\n</Configuration>\n")
	path := filepath.Join(os.TempDir(), "crescent-office.xml")
	return path, os.WriteFile(path, []byte(b.String()), 0o644)
}

type Payload struct {
	Apps []App `json:"apps"`
	// AutoUpdate adds a weekly Windows task that runs "winget upgrade --all".
	AutoUpdate bool `json:"autoUpdate,omitempty"`
	// Scripts run after all apps are installed, in this order (see script.go).
	Scripts []Script `json:"scripts,omitempty"`
}

const updateTaskName = `Crescent\Weekly app updates`

// scheduleUpdates registers a weekly task for the current user that upgrades
// every app winget knows about. A missed run (PC off) runs at the next chance.
func scheduleUpdates(winget string) error {
	args := fmt.Sprintf(`/c start "Crescent updates" /min "%s" upgrade --all --silent --accept-package-agreements --accept-source-agreements`, winget)
	var x strings.Builder
	xml.EscapeText(&x, []byte(args))
	task := `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Description>Keeps apps installed by Crescent up to date.</Description></RegistrationInfo>
  <Triggers>
    <CalendarTrigger>
      <StartBoundary>2026-01-04T12:00:00</StartBoundary>
      <ScheduleByWeek><DaysOfWeek><Sunday /></DaysOfWeek><WeeksInterval>1</WeeksInterval></ScheduleByWeek>
    </CalendarTrigger>
  </Triggers>
  <Principals><Principal id="Author"><LogonType>InteractiveToken</LogonType><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  <Settings>
    <StartWhenAvailable>true</StartWhenAvailable>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <RunOnlyIfNetworkAvailable>true</RunOnlyIfNetworkAvailable>
    <ExecutionTimeLimit>PT4H</ExecutionTimeLimit>
  </Settings>
  <Actions Context="Author"><Exec><Command>cmd.exe</Command><Arguments>` + x.String() + `</Arguments></Exec></Actions>
</Task>
`
	// schtasks reads task XML reliably only as UTF-16 LE with a byte order mark.
	u := utf16.Encode([]rune(task))
	buf := make([]byte, 2+2*len(u))
	buf[0], buf[1] = 0xFF, 0xFE
	for i, c := range u {
		binary.LittleEndian.PutUint16(buf[2+2*i:], c)
	}
	path := filepath.Join(os.TempDir(), "crescent-update-task.xml")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return err
	}
	defer os.Remove(path)
	cmd := exec.Command("schtasks", "/Create", "/TN", updateTaskName, "/XML", path, "/F")
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// errNoPayload means this is the plain program with no app list: the web installer agent.
var errNoPayload = errors.New("no app list found in this installer")

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]{1,99}$`)

// storeID matches Microsoft Store product IDs such as 9NKSQGP7F2NH (WhatsApp), which install
// from winget's msstore source instead of the community winget source.
var storeID = regexp.MustCompile(`^[0-9A-Z]{12}$`)

func readPayload(exe []byte) (*Payload, error) {
	n := len(exe)
	if n < len(magic)+4 || string(exe[n-len(magic):]) != magic {
		return nil, errNoPayload
	}
	size := int(binary.LittleEndian.Uint32(exe[n-len(magic)-4 : n-len(magic)]))
	start := n - len(magic) - 4 - size
	if size <= 0 || start < 0 {
		return nil, errors.New("the app list in this installer is damaged")
	}
	var p Payload
	if err := json.Unmarshal(exe[start:start+size], &p); err != nil {
		return nil, fmt.Errorf("the app list in this installer is damaged: %w", err)
	}
	return &p, nil
}

// loadApps reads the app list appended to this .exe and keeps only valid entries.
func loadApps() (*Payload, []App, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't locate this installer file: %w", err)
	}
	exe, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't read this installer file: %w", err)
	}
	return parsePayload(exe)
}

// parsePayload reads the list appended to exe and keeps only valid apps and scripts.
func parsePayload(exe []byte) (*Payload, []App, error) {
	payload, err := readPayload(exe)
	if err != nil {
		return nil, nil, fmt.Errorf("this installer has %w. Build a new one from the Crescent page", err)
	}
	return cleanPayload(payload)
}

// cleanPayload keeps only the valid apps and scripts of a payload.
func cleanPayload(payload *Payload) (*Payload, []App, error) {
	payload.Scripts = cleanScripts(payload.Scripts)
	apps := cleanApps(payload.Apps)
	if len(apps) == 0 {
		return nil, nil, errors.New("this installer has no apps selected. Build a new one from the Crescent page")
	}
	return payload, apps, nil
}

// macInstaller is true in the Mac build, where an app's ID is its Homebrew package.
var macInstaller = runtime.GOOS == "darwin"

// cleanApps keeps only valid entries, moves add-ons after the apps they need and apps that need
// the user's clicks to the end.
func cleanApps(list []App) []App {
	var apps, extras, direct []App
	for _, a := range list {
		var office []string
		for _, o := range a.Office {
			for _, p := range officePickable {
				if o == p {
					office = append(office, p)
				}
			}
		}
		a.Office = office
		// The window only shows icons the page made itself: small inline PNG or SVG images.
		if !strings.HasPrefix(a.Icon, "data:image/png;base64,") && !strings.HasPrefix(a.Icon, "data:image/svg+xml;base64,") {
			a.Icon = ""
		}
		if len(a.Detect) > 100 || strings.ContainsAny(a.Detect, "\\\x00") {
			a.Detect = ""
		}
		if isAddon(a.ID) {
			a.URL, a.Fixed = "", true
			if a.Name == "" {
				a.Name = a.ID
			}
			extras = append(extras, a)
			continue
		}
		if macInstaller {
			// Homebrew always installs to the usual place, and makers' Windows setups don't apply.
			a.URL, a.Fixed = "", true
			if !validBrew(a.ID) {
				continue
			}
		} else if !idPattern.MatchString(a.ID) || (a.URL != "" && !validDirectURL(a.URL)) {
			continue
		}
		if a.Name == "" {
			a.Name = a.ID
		}
		// Store apps, Microsoft 365 (Click-to-Run) and makers' own setups never take a folder from winget.
		a.Fixed = a.Fixed || storeID.MatchString(a.ID) || a.ID == "Microsoft.Office" || a.URL != ""
		if a.URL != "" {
			direct = append(direct, a) // these need the user's clicks, so they go last
		} else {
			apps = append(apps, a)
		}
	}
	return append(append(apps, extras...), direct...)
}

// installFolder matches a folder on a drive of this PC, such as D:\Program Files\Steam, without the
// characters Windows forbids in names (which also keeps it from breaking winget's quoting).
var installFolder = regexp.MustCompile(`^[A-Za-z]:(\\[^\\/:*?"<>|\x00-\x1f]+)+$`)

func validFolder(p string) bool {
	if len(p) > 200 || !installFolder.MatchString(p) {
		return false
	}
	for _, part := range strings.Split(p[3:], `\`) {
		if strings.Trim(part, ". ") == "" || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.HasPrefix(part, " ") {
			return false
		}
	}
	return true
}

// setLocations applies the folders chosen in the installer window: locs[i] is the folder for
// apps[i], or "" for its usual place. Fixed apps and invalid folders keep the usual place.
func setLocations(apps []App, locs []string) {
	for i := range apps {
		apps[i].Location = ""
		if i < len(locs) && !apps[i].Fixed && validFolder(locs[i]) {
			apps[i].Location = locs[i]
		}
	}
}

// UI is what the installer reports to while it works: the window on Windows,
// plain text elsewhere (used for testing).
type UI interface {
	Progress(done, total int, status string)
	Outcome(i int, outcome string) // installed, skipped, failed or cancelled
	Log(line string)
}

type Result struct {
	Installed, Skipped int
	Failed, Notes      []string
	Cancelled          bool
}

// Installer runs winget (or Homebrew on the Mac) once per app. Cancel stops the running install
// and skips the rest.
type Installer struct {
	Winget    string
	Brew      string // set on the Mac
	cancelled atomic.Bool
	mu        sync.Mutex
	current   *exec.Cmd
	manual    bool // current is a maker's setup the user is clicking through
}

func (in *Installer) Cancel() {
	in.cancelled.Store(true)
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.current != nil && in.current.Process != nil && !in.manual {
		in.current.Process.Kill()
	}
}

var (
	ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)
	// winget redraws spinners and download bars in place; those lines are noise in a log.
	noise = regexp.MustCompile(`^[-\\|/]$|[\x{2580}-\x{259F}]`)
)

// splitLines treats a carriage return like a newline, since winget rewrites its progress lines with \r.
func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// runLogged runs cmd as the current install, passing each line of its output to log, and waits.
func (in *Installer) runLogged(cmd *exec.Cmd, log func(string)) error {
	out, err := cmd.StdoutPipe()
	if err == nil {
		cmd.Stderr = cmd.Stdout
		in.mu.Lock()
		in.current = cmd
		err = cmd.Start()
		in.mu.Unlock()
	}
	if err == nil {
		sc := bufio.NewScanner(out)
		sc.Split(splitLines)
		for sc.Scan() {
			line := strings.TrimSpace(ansi.ReplaceAllString(sc.Text(), ""))
			if line != "" && !noise.MatchString(line) {
				log(line)
			}
		}
		err = cmd.Wait()
	}
	in.mu.Lock()
	in.current = nil
	in.mu.Unlock()
	return err
}

func (in *Installer) Run(apps []App, ui UI) Result {
	var r Result
	for i, a := range apps {
		if in.cancelled.Load() {
			r.Cancelled = true
			break
		}
		ui.Progress(i, len(apps), fmt.Sprintf("Installing %s (%d of %d)", a.Name, i+1, len(apps)))
		ui.Log(fmt.Sprintf("--- %s ---", a.Name))
		if isAddon(a.ID) {
			in.runAddon(i, a, ui, &r)
			continue
		}
		if in.Brew != "" {
			in.runBrew(i, a, ui, &r)
			continue
		}
		if a.URL != "" {
			in.runDirect(i, len(apps), a, ui, &r)
			continue
		}
		source := "winget"
		if storeID.MatchString(a.ID) {
			source = "msstore"
		}
		args := []string{"install", "--id", a.ID, "--exact", "--silent", "--disable-interactivity",
			"--source", source, "--accept-package-agreements", "--accept-source-agreements"}
		if a.Location != "" {
			args = append(args, "--location", a.Location)
			ui.Log("Folder: " + a.Location)
		}
		var officeCfg string
		if a.ID == "Microsoft.Office" && len(a.Office) > 0 {
			cfg, err := officeConfig(a.Office)
			officeCfg = cfg
			if err != nil {
				os.Remove(cfg)
				ui.Log("Couldn't prepare the Office setup: " + err.Error())
				ui.Outcome(i, "failed")
				r.Failed = append(r.Failed, a.Name)
				continue
			}
			args = append(args, "--override", "/configure \""+cfg+"\"")
		}
		if extra, ok := extraArgs[a.ID]; ok {
			args = append(args, extra...)
		}
		pgPassword := ""
		if strings.HasPrefix(a.ID, "PostgreSQL.PostgreSQL.") {
			pgPassword = randomPassword()
			args = append(args, "--custom", "--superpassword "+pgPassword)
		}

		cmd := exec.Command(in.Winget, args...)
		hideConsole(cmd)
		err := in.runLogged(cmd, ui.Log)
		if officeCfg != "" {
			os.Remove(officeCfg) // winget is done with it
		}

		var exitErr *exec.ExitError
		switch {
		case in.cancelled.Load():
			ui.Log("Cancelled.")
			ui.Outcome(i, "cancelled")
			r.Cancelled = true
		case err == nil:
			r.Installed++
			ui.Outcome(i, "installed")
			if pgPassword != "" {
				r.Notes = append(r.Notes, savePostgresPassword(a.Name, pgPassword))
			}
		case errors.As(err, &exitErr) && (uint32(exitErr.ExitCode()) == errAlreadyInstalled || uint32(exitErr.ExitCode()) == errUpdateNotApplied):
			ui.Log("Already installed, skipped.")
			ui.Outcome(i, "skipped")
			r.Skipped++
		default:
			if errors.As(err, &exitErr) {
				ui.Log(fmt.Sprintf("Failed (winget exit code 0x%X)", uint32(exitErr.ExitCode())))
			} else {
				ui.Log("Failed: " + err.Error())
			}
			ui.Outcome(i, "failed")
			r.Failed = append(r.Failed, a.Name)
		}
	}
	ui.Progress(len(apps), len(apps), "")
	return r
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Summary is the one-line outcome shown at the end.
func (r Result) Summary() string {
	s := fmt.Sprintf("Installed: %d   Already there: %d   Failed: %d", r.Installed, r.Skipped, len(r.Failed))
	if r.Cancelled {
		s += "   (stopped early)"
	}
	return s
}
