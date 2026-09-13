package cli

import "golang.org/x/sys/windows"

// isTerminal reports whether fd refers to a terminal.
func isTerminal(fd int) bool {
	var mode uint32
	err := windows.GetConsoleMode(windows.Handle(fd), &mode)
	return err == nil
}
