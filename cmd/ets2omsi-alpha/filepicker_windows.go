//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

type openFileName struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}

func platformPickSCS() (string, error) {
	dll := syscall.NewLazyDLL("comdlg32.dll")
	proc := dll.NewProc("GetOpenFileNameW")
	buf := make([]uint16, 32768)
	filter, _ := syscall.UTF16PtrFromString("ETS2 Packages (*.scs;*.zip)\x00*.scs;*.zip\x00All files (*.*)\x00*.*\x00\x00")
	title, _ := syscall.UTF16PtrFromString("Select ETS2 SCS package")
	ofn := openFileName{lStructSize: uint32(unsafe.Sizeof(openFileName{})), lpstrFilter: filter, lpstrFile: &buf[0], nMaxFile: uint32(len(buf)), lpstrTitle: title, flags: 0x00001000 | 0x00000800 | 0x00000008}
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return "", fmt.Errorf("file selection cancelled")
	}
	return syscall.UTF16ToString(buf), nil
}

type browseInfo struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

func platformPickFolder(titleText string) (string, error) {
	shell := syscall.NewLazyDLL("shell32.dll")
	browse := shell.NewProc("SHBrowseForFolderW")
	getPath := shell.NewProc("SHGetPathFromIDListW")
	ole := syscall.NewLazyDLL("ole32.dll")
	free := ole.NewProc("CoTaskMemFree")
	title, _ := syscall.UTF16PtrFromString(titleText)
	display := make([]uint16, 260)
	bi := browseInfo{pszDisplayName: &display[0], lpszTitle: title, ulFlags: 0x00000040 | 0x00000001} // BIF_NEWDIALOGSTYLE | RETURNONLYFSDIRS
	pidl, _, _ := browse.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", fmt.Errorf("folder selection cancelled")
	}
	defer free.Call(pidl)
	buf := make([]uint16, 32768)
	ok, _, _ := getPath.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
	if ok == 0 {
		return "", fmt.Errorf("unable to resolve selected folder")
	}
	return syscall.UTF16ToString(buf), nil
}
