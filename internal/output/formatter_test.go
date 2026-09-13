package output

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.in); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "30 seconds"},
		{5 * time.Minute, "5 minutes"},
		{3 * time.Hour, "3 hours"},
		{48 * time.Hour, "2 days"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.in); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	if got := formatTime(time.Time{}); got != "unknown" {
		t.Errorf("formatTime(zero) = %q, want unknown", got)
	}
	tm := time.Date(2024, 1, 1, 14, 32, 1, 0, time.UTC)
	if got := formatTime(tm); got != "14:32:01" {
		t.Errorf("formatTime = %q, want 14:32:01", got)
	}
}

func TestFormatStarted(t *testing.T) {
	if got := formatStarted(time.Time{}); got != "unknown" {
		t.Errorf("formatStarted(zero) = %q, want unknown", got)
	}
	recent := time.Now().Add(-5 * time.Minute)
	if got := formatStarted(recent); got != "5 minutes ago" {
		t.Errorf("formatStarted(recent) = %q, want '5 minutes ago'", got)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"toolongstring", 8, "toolong…"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		if got := truncate(tt.in, tt.max); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

func TestShortenPath(t *testing.T) {
	// shortenPath depends on os.UserHomeDir(); just verify it doesn't panic
	// and returns something for both empty and normal input.
	if got := shortenPath(""); got != "" {
		t.Errorf("shortenPath(\"\") = %q, want empty", got)
	}
	if got := shortenPath("/some/random/path"); got == "" {
		t.Errorf("shortenPath returned empty for non-empty input")
	}
}

func TestIsSystemOrBackgroundProcess(t *testing.T) {
	system := []string{
		// Windows
		"svchost.exe", "SYSTEM", "lsass.exe",
		// macOS
		"ControlCenter", "rapportd", "coreaudiod", "Code Helper", "Code Helper (Plugin)",
		// Linux
		"systemd", "dbus-daemon", "cron",
	}
	for _, name := range system {
		if !IsSystemOrBackgroundProcess(name) {
			t.Errorf("IsSystemOrBackgroundProcess(%q) = false, want true", name)
		}
	}

	notSystem := []string{"node", "python", "Code Helper (GPU) extra", "myapp"}
	for _, name := range notSystem {
		if IsSystemOrBackgroundProcess(name) {
			t.Errorf("IsSystemOrBackgroundProcess(%q) = true, want false", name)
		}
	}
}
