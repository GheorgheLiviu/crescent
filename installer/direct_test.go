//go:build !windows

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const magicianURL = "https://download.semiconductor.samsung.com/resources/software-resources/Samsung_Magician_Installer_Official_9.0.1.950.exe"

type recUI struct {
	mu       sync.Mutex
	outcomes map[int]string
	log      []string
	status   []string
}

func (u *recUI) Progress(done, total int, s string) {
	u.mu.Lock()
	u.status = append(u.status, s)
	u.mu.Unlock()
}
func (u *recUI) Outcome(i int, o string) { u.mu.Lock(); u.outcomes[i] = o; u.mu.Unlock() }
func (u *recUI) Log(l string)            { u.mu.Lock(); u.log = append(u.log, l); u.mu.Unlock() }

// fakeSamsung serves handler over TLS with a certificate for Samsung's download host (and
// other.example, for redirect tests) and points curl at it whatever host a link names.
func fakeSamsung(t *testing.T, handler http.HandlerFunc) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		DNSNames:  []string{"download.semiconductor.samsung.com", "other.example"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca := filepath.Join(t.TempDir(), "ca.pem")
	os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
	srv := httptest.NewUnstartedServer(handler)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	curlTestArgs = []string{"--noproxy", "*", "--cacert", ca, "--connect-to", "::" + srv.Listener.Addr().String()}
	t.Cleanup(func() { curlTestArgs = nil })
}

func setupScript(exit int) string { return fmt.Sprintf("#!/bin/sh\nexit %d\n", exit) }

func TestValidDirectURL(t *testing.T) {
	for raw, want := range map[string]bool{
		magicianURL: true,
		"http://download.semiconductor.samsung.com/a.exe":               false,
		"https://download.semiconductor.samsung.com:8443/a.exe":         false,
		"https://evil.example/a.exe":                                    false,
		"https://user@download.semiconductor.samsung.com/a.exe":         false,
		"https://download.semiconductor.samsung.com/a.zip":              false,
		"https://download.semiconductor.samsung.com.evil.example/a.exe": false,
	} {
		if got := validDirectURL(raw); got != want {
			t.Errorf("validDirectURL(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestCleanAppsPutsManualAppsLast(t *testing.T) {
	got := cleanApps([]App{
		{ID: "Samsung.Magician", Name: "Samsung Magician", URL: magicianURL, Detect: "Samsung Magician"},
		{ID: "Google.Chrome", Name: "Google Chrome"},
		{ID: "Bad.Link", Name: "Bad", URL: "https://evil.example/x.exe"},
		{ID: "7zip.7zip"},
	})
	var ids []string
	for _, a := range got {
		ids = append(ids, a.ID)
	}
	if strings.Join(ids, ",") != "Google.Chrome,7zip.7zip,Samsung.Magician" {
		t.Fatalf("order = %v", ids)
	}
	if got[1].Name != "7zip.7zip" {
		t.Errorf("missing name not filled from ID: %q", got[1].Name)
	}
}

func TestRunDirect(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		outcome string
		logHas  string
	}{
		{"installs", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, setupScript(0)) }, "installed", "setup window is open"},
		{"user closes setup", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, setupScript(2)) }, "failed", "closed before it finished"},
		{"setup error", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, setupScript(3)) }, "failed", "setup exit code 3"},
		{"link gone", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, "failed", "no longer works"},
		{"server trouble", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "busy", http.StatusForbidden) }, "failed", "server had a problem (HTTP 403)"},
		{"redirect elsewhere", func(w http.ResponseWriter, r *http.Request) {
			if r.Host == "other.example" {
				fmt.Fprint(w, setupScript(0))
				return
			}
			http.Redirect(w, r, "https://other.example/x.exe", http.StatusFound)
		}, "failed", "somewhere other than the maker"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeSamsung(t, c.handler)
			ui := &recUI{outcomes: map[int]string{}}
			in := &Installer{Winget: "true"}
			r := in.Run([]App{{ID: "Samsung.Magician", Name: "Samsung Magician", URL: magicianURL}}, ui)
			if ui.outcomes[0] != c.outcome {
				t.Fatalf("outcome = %q, want %q; log: %v", ui.outcomes[0], c.outcome, ui.log)
			}
			if !strings.Contains(strings.Join(ui.log, "\n"), c.logHas) {
				t.Errorf("log lacks %q: %v", c.logHas, ui.log)
			}
			if (c.outcome == "installed") != (r.Installed == 1) {
				t.Errorf("result = %+v", r)
			}
		})
	}
}

func TestDownloadReportsProgressAndCancels(t *testing.T) {
	body := strings.Repeat("x", 1<<20)
	in := &Installer{}
	fakeSamsung(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		fmt.Fprint(w, body)
	})
	var pcts []int
	file, err := in.download(magicianURL, func(p int) { pcts = append(pcts, p) })
	if err != nil || !strings.HasSuffix(file, "Samsung_Magician_Installer_Official_9.0.1.950.exe") {
		t.Fatalf("download = %q, %v", file, err)
	}
	if len(pcts) == 0 || pcts[len(pcts)-1] != 100 {
		t.Errorf("progress = %v", pcts)
	}
	in.Cancel()
	if _, err := in.download(magicianURL, func(int) {}); err != errCancelled {
		t.Errorf("after Cancel, download err = %v", err)
	}
}

// Cancel while the maker's setup is open must not kill it: the user finishes or closes it.
func TestCancelWaitsForManualSetup(t *testing.T) {
	for _, c := range []struct {
		exit    int
		outcome string
	}{{0, "installed"}, {2, "cancelled"}} {
		t.Run(c.outcome, func(t *testing.T) {
			fakeSamsung(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "#!/bin/sh\nsleep 1\nexit %d\n", c.exit)
			})
			ui := &recUI{outcomes: map[int]string{}}
			in := &Installer{Winget: "true"}
			go func() {
				for {
					ui.mu.Lock()
					w := ui.outcomes[0] == "waiting"
					ui.mu.Unlock()
					if w {
						in.Cancel()
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
			}()
			r := in.Run([]App{{ID: "Samsung.Magician", Name: "Samsung Magician", URL: magicianURL}}, ui)
			if ui.outcomes[0] != c.outcome {
				t.Fatalf("outcome = %q, want %q; log: %v", ui.outcomes[0], c.outcome, ui.log)
			}
			if c.outcome == "installed" && r.Installed != 1 {
				t.Errorf("result = %+v", r)
			}
		})
	}
}
