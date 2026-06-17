//go:build !windows

package ui

import (
	"os"
	"os/exec"
)

// disableEcho turns off terminal echo via stty (zero-dependency). Returns a
// restore func and whether echo was actually disabled. No-ops when stdin is not
// a terminal or stty is unavailable.
func disableEcho() (restore func(), ok bool) {
	if fi, err := os.Stdin.Stat(); err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		return func() {}, false
	}
	if err := sttyRun("-echo"); err != nil {
		return func() {}, false
	}
	return func() { _ = sttyRun("echo") }, true
}

func sttyRun(arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
