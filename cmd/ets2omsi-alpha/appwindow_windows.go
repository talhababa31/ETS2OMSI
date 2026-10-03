//go:build windows

package main

import (
	"log"
	"path/filepath"
	"runtime"

	"github.com/jchv/go-webview2"
)

// The WebView2 window must own the process main thread.
func init() { runtime.LockOSThread() }

// platformRunWindow shows the UI inside a native desktop window (Edge
// WebView2, preinstalled on Windows 10/11). It blocks until the window is
// closed or quit is signalled. It returns false when WebView2 is unavailable
// so the caller can fall back to the browser.
func platformRunWindow(url, dataDir string, quit <-chan struct{}) bool {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(dataDir, "webview2"),
		WindowOptions: webview2.WindowOptions{
			Title:  "ETS2 → OMSI 2 AI Converter " + appVersion,
			Width:  1440,
			Height: 900,
			Center: true,
		},
	})
	if w == nil {
		log.Println("WebView2 runtime not available; falling back to browser")
		return false
	}
	go func() {
		<-quit
		w.Dispatch(func() { w.Terminate() })
	}()
	w.Navigate(url)
	w.Run()
	return true
}
