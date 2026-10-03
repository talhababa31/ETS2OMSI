//go:build !windows

package main

import "fmt"

func platformPickSCS() (string, error) {
	return "", fmt.Errorf("native file picker is available in the Windows build; enter a path manually")
}
func platformPickFolder(titleText string) (string, error) {
	return "", fmt.Errorf("native folder picker is available in the Windows build; enter a path manually")
}

// Non-Windows builds run in a terminal: the URL is printed on stdout and
// Ctrl+C / the UI "Kapat" button stops the server.
func platformStatusWindow(url, logPath string, open, quit func()) {}
func platformShowError(msg string)                                { fmt.Println(msg) }

func platformRunWindow(url, dataDir string, quit <-chan struct{}) bool { return false }

func platformShowInfo(msg string) { fmt.Println(msg) }
