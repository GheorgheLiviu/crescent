//go:build !windows && !darwin

// A plain-text front end so the installer logic can be tested off Windows.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

type textUI struct{}

func (textUI) Progress(done, total int, status string) {
	if status != "" {
		fmt.Printf("\n[%d/%d] %s\n", done, total, status)
	}
}
func (textUI) Log(line string)               { fmt.Println("  " + line) }
func (textUI) Outcome(i int, outcome string) { fmt.Printf("  => %s\n", outcome) }

func main() {
	payload, apps, err := loadApps()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	winget, err := exec.LookPath("winget")
	if err != nil {
		fmt.Println("winget was not found")
		os.Exit(1)
	}
	in := &Installer{Winget: winget}
	r := in.Run(apps, textUI{})
	fmt.Println("\n" + r.Summary())
	if payload.AutoUpdate {
		if err := scheduleUpdates(winget); err != nil {
			fmt.Println("Couldn't set up weekly updates:", err)
		}
	}
	for i := range payload.Scripts {
		note, _ := in.RunScript(&payload.Scripts[i], textUI{})
		r.Notes = append(r.Notes, note)
	}
	for _, n := range r.Notes {
		fmt.Println(n)
	}
}
