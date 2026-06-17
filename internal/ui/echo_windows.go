//go:build windows

package ui

// disableEcho is a no-op on Windows (no stty); secrets are read with echo.
func disableEcho() (restore func(), ok bool) { return func() {}, false }
