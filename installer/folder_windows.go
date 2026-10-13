package main

import (
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// pickFolder shows Windows' own folder picker (the Explorer-style one) owned by the installer
// window and returns the chosen folder, or "" if the user cancelled. It starts in start, or the
// nearest folder above it that exists, or This PC. It must run on the window's thread, outside
// any WebView2 event handler, since the picker runs its own message loop.
func pickFolder(owner uintptr, title, start string) string {
	var dlg unsafe.Pointer
	if hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOpenDialog)), uintptr(unsafe.Pointer(&dlg))); hr != 0 || dlg == nil {
		return ""
	}
	defer comCall(dlg, methodRelease)

	var opts uint32
	comCall(dlg, fileDialogGetOptions, uintptr(unsafe.Pointer(&opts)))
	comCall(dlg, fileDialogSetOptions, uintptr(opts|fosNoChangeDir|fosPickFolders|fosForceFileSystem|fosPathMustExist))
	if t, err := windows.UTF16PtrFromString(title); err == nil && title != "" {
		comCall(dlg, fileDialogSetTitle, uintptr(unsafe.Pointer(t)))
	}
	for _, dir := range append(parentFolders(start), thisPC) {
		if item := shellItem(dir); item != nil {
			comCall(dlg, fileDialogSetFolder, uintptr(item))
			comCall(item, methodRelease)
			break
		}
	}

	if comCall(dlg, modalWindowShow, owner) != 0 { // cancelled, or the picker failed
		return ""
	}
	var item unsafe.Pointer
	if comCall(dlg, fileDialogGetResult, uintptr(unsafe.Pointer(&item))) != 0 || item == nil {
		return ""
	}
	defer comCall(item, methodRelease)
	var name *uint16
	if comCall(item, shellItemGetDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&name))) != 0 || name == nil {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(name))
	return windows.UTF16PtrToString(name)
}

// parentFolders lists dir and each folder above it, nearest first.
func parentFolders(dir string) []string {
	var out []string
	for dir != "" && dir != "." {
		out = append(out, dir)
		up := filepath.Dir(dir)
		if up == dir {
			break
		}
		dir = up
	}
	return out
}

// shellItem returns an IShellItem for a folder path, or nil if it doesn't exist.
func shellItem(path string) unsafe.Pointer {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil
	}
	var item unsafe.Pointer
	if hr, _, _ := procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(p)), 0,
		uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item))); hr != 0 {
		return nil
	}
	return item
}

// comCall calls method number index of a COM object, whose first field points to its method table.
// uintptrescapes keeps variables passed as uintptr(unsafe.Pointer(&v)) alive and in place.
//
//go:uintptrescapes
func comCall(obj unsafe.Pointer, index int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return r
}

var (
	ole32                           = windows.NewLazySystemDLL("ole32.dll")
	shell32                         = windows.NewLazySystemDLL("shell32.dll")
	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")

	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidFileOpenDialog   = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidShellItem        = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

const (
	clsctxInprocServer = 0x1
	thisPC             = "::{20D04FE0-3AEA-1069-A2D8-08002B30309D}"
	sigdnFileSysPath   = 0x80058000

	fosNoChangeDir     = 0x8
	fosPickFolders     = 0x20
	fosForceFileSystem = 0x40
	fosPathMustExist   = 0x800

	// Method numbers in the IFileOpenDialog and IShellItem tables (shobjidl.h).
	methodRelease           = 2
	modalWindowShow         = 3
	fileDialogSetOptions    = 9
	fileDialogGetOptions    = 10
	fileDialogSetFolder     = 12
	fileDialogSetTitle      = 17
	fileDialogGetResult     = 20
	shellItemGetDisplayName = 5
)
