package platform

import (
	"fmt"
	"runtime"
)

// NewProvider returns the appropriate ProcessProvider for the current OS.
func NewProvider() (ProcessProvider, error) {
	switch runtime.GOOS {
	case "linux":
		return &LinuxProvider{}, nil
	case "darwin":
		return &MacOSProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s (Linux and macOS are supported in v0.1)", runtime.GOOS)
	}
}
