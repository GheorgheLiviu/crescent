package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Some apps aren't in winget and their setup can't run silently (Samsung Magician). For those the
// page passes the vendor's own download link. The installer fetches it from an allowed vendor
// server over HTTPS, checks that Windows trusts its digital signature, and opens the vendor's
// setup window for the user to click through. These apps always run after the silent ones.
var directHosts = map[string]bool{
	"download.semiconductor.samsung.com": true,
}

var safeFileName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}\.exe$`)

func validDirectURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" &&
		directHosts[u.Hostname()] && strings.HasSuffix(strings.ToLower(u.Path), ".exe")
}

// Inno Setup exit codes for a setup the user closed before it finished.
const (
	innoCancelledBeforeInstall = 2
	innoCancelledDuringInstall = 5
)

func (in *Installer) runDirect(i, total int, a App, ui UI, r *Result) {
	if a.Detect != "" && isInstalled(a.Detect) {
		ui.Log("Already installed, skipped.")
		ui.Outcome(i, "skipped")
		r.Skipped++
		return
	}
	file, err := in.download(a.URL, func(pct int) {
		ui.Progress(i, total, fmt.Sprintf("Downloading %s from its maker, %d%% (%d of %d)", a.Name, pct, i+1, total))
	})
	if err == nil {
		defer os.RemoveAll(filepath.Dir(file))
		ui.Log("Downloaded " + filepath.Base(file) + ". Checking its digital signature.")
		os.Chmod(file, 0o755) // lets the tests run it off Windows; no effect on Windows
		err = verifySignature(file)
	}
	ran := false
	if err == nil {
		// The setup is the user's to finish, so Cancel doesn't kill it (killing the Inno Setup
		// loader would leave its wizard window open anyway); Crescent stops once it closes.
		cmd := exec.Command(file)
		in.mu.Lock()
		if in.cancelled.Load() {
			err = errCancelled
		} else if err = cmd.Start(); err == nil {
			in.current, in.manual, ran = cmd, true, true
		}
		in.mu.Unlock()
		if ran {
			ui.Progress(i, total, fmt.Sprintf("Finish the %s setup window (%d of %d)", a.Name, i+1, total))
			ui.Outcome(i, "waiting")
			ui.Log(a.Name + " can't install on its own, so its setup window is open. Follow its steps; Crescent continues when it closes.")
			err = cmd.Wait()
			in.mu.Lock()
			in.current, in.manual = nil, false
			in.mu.Unlock()
		}
	}

	var exitErr *exec.ExitError
	closedEarly := errors.As(err, &exitErr) && (exitErr.ExitCode() == innoCancelledBeforeInstall || exitErr.ExitCode() == innoCancelledDuringInstall)
	switch {
	case ran && err == nil: // finished, even if Cancel was pressed while it was open
		r.Installed++
		ui.Outcome(i, "installed")
	case in.cancelled.Load() && (!ran || closedEarly):
		ui.Log("Cancelled.")
		ui.Outcome(i, "cancelled")
		r.Cancelled = true
	case closedEarly:
		ui.Log("The setup window was closed before it finished.")
		ui.Outcome(i, "failed")
		r.Failed = append(r.Failed, a.Name)
	default:
		if errors.As(err, &exitErr) {
			ui.Log(fmt.Sprintf("Failed (setup exit code %d)", exitErr.ExitCode()))
		} else {
			ui.Log("Failed: " + err.Error())
		}
		ui.Outcome(i, "failed")
		r.Failed = append(r.Failed, a.Name)
	}
}

var errCancelled = errors.New("cancelled")

// download saves the file at link into a new temporary folder and returns its path. It uses the
// curl.exe that ships with Windows 10 and 11 rather than Go's HTTP stack, which would double the
// size of the installer. progress gets the percentage whenever it changes.
func (in *Installer) download(link string, progress func(int)) (string, error) {
	u, err := url.Parse(link)
	if err != nil || !validDirectURL(link) {
		return "", errors.New("the download link isn't one Crescent accepts")
	}
	name := path.Base(u.Path)
	if !safeFileName.MatchString(name) {
		name = "setup.exe"
	}
	dir, err := os.MkdirTemp("", "Crescent-")
	if err != nil {
		return "", err
	}
	file := filepath.Join(dir, name)
	fail := func(err error) (string, error) {
		os.RemoveAll(dir)
		if in.cancelled.Load() {
			return "", errCancelled
		}
		return "", err
	}

	// --ssl-no-revoke: Windows' curl otherwise fails outright when antivirus HTTPS scanning or a
	// filtering firewall blocks the certificate revocation check. The host check below and the
	// signature check on the file still apply.
	args := append([]string{"--fail", "--location", "--max-redirs", "5", "--proto", "=https", "--proto-redir", "=https",
		"--ssl-no-revoke", "--connect-timeout", "30", "--retry", "2", "--progress-bar", "--output", file,
		"--write-out", "%{http_code} %{url_effective}"}, curlTestArgs...)
	cmd := exec.Command(curlPath(), append(args, link)...)
	hideConsole(cmd)
	var written strings.Builder
	cmd.Stdout = &written
	errOut, err := cmd.StderrPipe()
	if err == nil {
		in.mu.Lock()
		if in.cancelled.Load() {
			err = errCancelled
		} else {
			in.current = cmd
			err = cmd.Start()
		}
		in.mu.Unlock()
	}
	if err != nil {
		return fail(err)
	}
	var lastLine string
	last := -1
	sc := bufio.NewScanner(errOut)
	sc.Split(splitLines)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m := curlPercent.FindStringSubmatch(line); m != nil {
			if pct, _ := strconv.Atoi(m[1]); pct != last {
				last = pct
				progress(pct)
			}
		} else if line != "" {
			lastLine = line
		}
	}
	err = cmd.Wait()
	in.mu.Lock()
	in.current = nil
	in.mu.Unlock()
	code, final, _ := strings.Cut(strings.TrimSpace(written.String()), " ")
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 22 && (code == "404" || code == "410"): // --fail
		return fail(errors.New("the download link no longer works; the page needs a newer link"))
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 22:
		return fail(fmt.Errorf("the maker's server had a problem (HTTP %s). Run the installer again later", code))
	default:
		if lastLine != "" {
			return fail(fmt.Errorf("couldn't download it: %s", lastLine))
		}
		return fail(fmt.Errorf("couldn't download it: %w", err))
	}
	// Only run what came from the maker's server, even after redirects.
	if !validDirectURL(final) {
		return fail(errors.New("the download link pointed somewhere other than the maker's server"))
	}
	return file, nil
}

var curlPercent = regexp.MustCompile(`(\d{1,3})(?:[.,]\d)?%$`)

// curlTestArgs lets tests point curl at a local server.
var curlTestArgs []string

func curlPath() string {
	if root := os.Getenv("SystemRoot"); root != "" {
		return filepath.Join(root, "System32", "curl.exe")
	}
	return "curl"
}
