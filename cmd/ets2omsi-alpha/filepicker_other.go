//go:build !windows

package main

import "fmt"

func platformPickSCS() (string, error) {
	return "", fmt.Errorf("native file picker is available in the Windows build; enter a path manually")
}
func platformPickFolder(titleText string) (string, error) {
	return "", fmt.Errorf("native folder picker is available in the Windows build; enter a path manually")
}
