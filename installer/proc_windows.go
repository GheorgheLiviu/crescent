package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// hideConsole keeps winget and other helpers from flashing a console window.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// verifySignature asks Windows whether it trusts the file's digital signature, the same check
// Windows makes before it runs a signed program.
func verifySignature(file string) error {
	p, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	info := windows.WinTrustFileInfo{Size: uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})), FilePath: p}
	data := windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_NONE,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&info),
	}
	err = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	runtime.KeepAlive(&info)
	if err != nil {
		return fmt.Errorf("Windows doesn't trust its digital signature (%v), so it wasn't opened", err)
	}
	return nil
}

// isInstalled reports whether Installed apps lists a program whose name starts with displayName.
func isInstalled(displayName string) bool {
	want := strings.ToLower(displayName)
	views := []struct {
		root   registry.Key
		access uint32
	}{
		{registry.LOCAL_MACHINE, registry.WOW64_64KEY},
		{registry.LOCAL_MACHINE, registry.WOW64_32KEY},
		{registry.CURRENT_USER, 0},
	}
	for _, v := range views {
		k, err := registry.OpenKey(v.root, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.ENUMERATE_SUB_KEYS|v.access)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		found := false
		for _, n := range names {
			sk, err := registry.OpenKey(k, n, registry.QUERY_VALUE|v.access)
			if err != nil {
				continue
			}
			dn, _, _ := sk.GetStringValue("DisplayName")
			sk.Close()
			if dn != "" && strings.HasPrefix(strings.ToLower(dn), want) {
				found = true
				break
			}
		}
		k.Close()
		if found {
			return true
		}
	}
	return false
}
