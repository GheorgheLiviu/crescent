package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

const createNewConsole = 0x00000010 // CREATE_NEW_CONSOLE

var scriptKinds = map[string]bool{"ps1": true, "cmd": true}

const scriptOwnWindow = true

// scriptCommand runs the script with Windows' own PowerShell or Command Prompt, by full path so a
// program of the same name elsewhere on the Path can't stand in. The script gets a visible
// console window of its own (no hideConsole).
var scriptCommand = func(file, kind string) *exec.Cmd {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	if kind == "ps1" {
		cmd := exec.Command(filepath.Join(root, `System32\WindowsPowerShell\v1.0\powershell.exe`),
			"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", file)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
		return cmd
	}
	cmd := exec.Command(filepath.Join(root, `System32\cmd.exe`), "/d", "/c", file)
	// cmd.exe strips the quotes around a path that has spaces and brackets, such as
	// "setup (1).cmd"; with /s and the whole command quoted once more, they survive.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole,
		CmdLine: syscall.EscapeArg(cmd.Path) + ` /d /s /c ""` + file + `""`}
	return cmd
}

// startScript starts cmd in a console window of its own, titled title. os/exec can't: it always
// hands the new process standard handles (the null device when none are set), so the window
// would stay blank and ignore the keyboard. Started without them, the process uses its window.
func startScript(cmd *exec.Cmd, title string) error {
	if cmd.Err != nil {
		return cmd.Err
	}
	sys := cmd.SysProcAttr
	if sys == nil {
		sys = &syscall.SysProcAttr{CreationFlags: createNewConsole}
	}
	line := sys.CmdLine
	if line == "" {
		args := make([]string, len(cmd.Args))
		for i, a := range cmd.Args {
			args[i] = syscall.EscapeArg(a)
		}
		line = strings.Join(args, " ")
	}
	app, err := syscall.UTF16PtrFromString(cmd.Path)
	if err != nil {
		return err
	}
	cmdLine, err := syscall.UTF16PtrFromString(line)
	if err != nil {
		return err
	}
	var dir *uint16
	if cmd.Dir != "" {
		if dir, err = syscall.UTF16PtrFromString(cmd.Dir); err != nil {
			return err
		}
	}
	var envBlock *uint16
	if cmd.Env != nil {
		var block []uint16
		for _, kv := range cmd.Env {
			if strings.ContainsRune(kv, 0) {
				continue
			}
			block = append(append(block, utf16.Encode([]rune(kv))...), 0)
		}
		if len(block) == 0 {
			block = append(block, 0)
		}
		block = append(block, 0) // the block ends with an empty entry
		envBlock = &block[0]
	}
	si := &syscall.StartupInfo{}
	si.Cb = uint32(unsafe.Sizeof(*si))
	si.Title, _ = syscall.UTF16PtrFromString(title)
	var pi syscall.ProcessInformation
	err = syscall.CreateProcess(app, cmdLine, nil, nil, false,
		sys.CreationFlags|syscall.CREATE_UNICODE_ENVIRONMENT, envBlock, dir, si, &pi)
	if err != nil {
		return &os.PathError{Op: "start", Path: cmd.Path, Err: err}
	}
	defer syscall.CloseHandle(pi.Thread)
	defer syscall.CloseHandle(pi.Process) // held until FindProcess has its own handle, so the id stays this process's
	p, err := os.FindProcess(int(pi.ProcessId))
	if err != nil {
		return err
	}
	cmd.Process = p // cmd.Wait works on it like on a process cmd.Start started
	return nil
}

// freshEnv is the environment a newly opened window would get, read from the registry, so the
// script sees the Path and other variables the apps just installed have set.
func freshEnv() []string {
	return mergeEnv(os.Environ(),
		regEnv(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`),
		regEnv(registry.CURRENT_USER, `Environment`))
}

func regEnv(root registry.Key, path string) []envVar {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, _ := k.ReadValueNames(-1)
	var vars []envVar
	for _, n := range names {
		v, typ, err := k.GetStringValue(n)
		if err != nil {
			continue // not text
		}
		vars = append(vars, envVar{name: n, value: v, expand: typ == registry.EXPAND_SZ})
	}
	return vars
}
