//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	mbYesNo          = 0x00000004
	mbOK             = 0x00000000
	mbIconInfo       = 0x00000040
	mbIconError      = 0x00000010
	mbSetForeground  = 0x00010000
	idYes            = 6
	idNo             = 7
	statusWindowName = "ETS2OMSI V2.2"
)

var procMessageBoxW = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")

func messageBox(text, title string, flags uintptr) int {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(title)
	r, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), flags)
	return int(r)
}

// platformStatusWindow is the visible window of the GUI build (which has no
// console). It shows the UI address and log path and offers an exit path, so
// the program is never an invisible background server.
func platformStatusWindow(url, logPath string, open, quit func()) {
	text := "ETS2OMSI çalışıyor.\n\nArayüz adresi:\n" + url +
		"\n\nTarayıcı açılmadıysa bu adresi tarayıcına yaz.\nLog: " + logPath +
		"\n\nEvet  = arayüzü tarayıcıda (yeniden) aç\nHayır = programı kapat"
	for {
		switch messageBox(text, statusWindowName, mbYesNo|mbIconInfo|mbSetForeground) {
		case idYes:
			open()
		case idNo:
			quit()
			return
		default:
			// MessageBox unavailable: keep serving; URL is in the log.
			return
		}
	}
}

func platformShowError(msg string) {
	messageBox(msg, statusWindowName, mbOK|mbIconError|mbSetForeground)
}
