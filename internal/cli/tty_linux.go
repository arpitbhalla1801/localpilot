package cli

import "golang.org/x/sys/unix"

// isTerminal reports whether fd refers to a terminal.
func isTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}
