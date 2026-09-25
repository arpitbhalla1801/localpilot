package output

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// what it wrote, for testing Print* functions that write directly to
// os.Stdout rather than an injectable io.Writer.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	original := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = original
	w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(out)
}

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
		if got := FormatBytes(tt.in); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
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
	if got := FormatTime(time.Time{}); got != "unknown" {
		t.Errorf("FormatTime(zero) = %q, want unknown", got)
	}
	tm := time.Date(2024, 1, 1, 14, 32, 1, 0, time.UTC)
	if got := FormatTime(tm); got != "14:32:01" {
		t.Errorf("FormatTime = %q, want 14:32:01", got)
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"normal text", "normal text"},
		{"evil\x1b[31mRED\x1b[0m", "evil\\x1b[31mRED\\x1b[0m"},
		{"tab\ttab", "tab\\x09tab"},
		{"line1\nline2", "line1\\x0aline2"},
		{"del\x7fchar", "del\\x7fchar"},
	}
	for _, tt := range tests {
		got := sanitize(tt.in)
		if got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if strings.ContainsAny(got, "\x1b\t\n\x7f") {
			t.Errorf("sanitize(%q) = %q still contains a raw control character", tt.in, got)
		}
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
		{"emoji_😀😃😄😁😆_dir", 10, "emoji_😀😃😄…"},
		{"café_naïve_résumé", 8, "café_na…"},
	}
	for _, tt := range tests {
		got := Truncate(tt.in, tt.max)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("Truncate(%q, %d) = %q, which is not valid UTF-8", tt.in, tt.max, got)
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

// TestPrintKillConfirmation_NilProcess is the regression test for #34:
// binding.Process == nil used to be dereferenced unconditionally in this
// exact branch, guaranteeing a panic. It must instead print a fallback
// message and return normally.
func TestPrintKillConfirmation_NilProcess(t *testing.T) {
	binding := &models.PortBinding{Port: 3000, InUse: true, Process: nil}

	out := captureStdout(t, func() {
		PrintKillConfirmation(binding)
	})

	if !strings.Contains(out, "3000") {
		t.Errorf("PrintKillConfirmation with nil Process output = %q, want it to mention the port", out)
	}
}

func TestPrintKillConfirmation_WithProcess(t *testing.T) {
	binding := &models.PortBinding{
		Port:    3000,
		InUse:   true,
		Process: &models.Process{PID: 1234, Name: "node"},
	}

	out := captureStdout(t, func() {
		PrintKillConfirmation(binding)
	})

	if !strings.Contains(out, "1234") || !strings.Contains(out, "node") {
		t.Errorf("PrintKillConfirmation output = %q, want it to mention PID 1234 and name node", out)
	}
}

// TestPrintDashboard_ShowsContainerName is the regression test for #37:
// the dashboard (bare `localpilot`) used to show the raw local process
// name (e.g. docker-proxy/com.docker.backend) for a container-published
// port, unlike list/port/inspect, which all resolve it to the container's
// own name.
func TestPrintDashboard_ShowsContainerName(t *testing.T) {
	ports := []models.Port{
		{
			Number:    3000,
			Address:   "localhost",
			Process:   &models.Process{PID: 1, Name: "com.docker.backend"},
			Container: &models.Container{ID: "abc123", Name: "my-app", Image: "nginx:alpine"},
		},
	}

	out := captureStdout(t, func() {
		PrintDashboard(ports)
	})

	if !strings.Contains(out, "my-app") {
		t.Errorf("PrintDashboard output = %q, want it to mention container name my-app", out)
	}
	if strings.Contains(out, "com.docker.backend") {
		t.Errorf("PrintDashboard output = %q, want the generic backend process name replaced, not also shown", out)
	}
}

func TestPrintDashboard_NoContainerShowsProcessName(t *testing.T) {
	ports := []models.Port{
		{
			Number:  3000,
			Address: "localhost",
			Process: &models.Process{PID: 1, Name: "node"},
		},
	}

	out := captureStdout(t, func() {
		PrintDashboard(ports)
	})

	if !strings.Contains(out, "node") {
		t.Errorf("PrintDashboard output = %q, want it to mention process name node", out)
	}
}

func TestPrintDockerList(t *testing.T) {
	out := captureStdout(t, func() {
		PrintDockerList([]*models.Container{
			{Name: "web", Image: "nginx:latest", Ports: []int{80, 443}},
		})
	})
	for _, want := range []string{"web", "nginx:latest", "80, 443"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	if out := captureStdout(t, func() { PrintDockerList(nil) }); !strings.Contains(out, "No running containers") {
		t.Errorf("empty output = %q", out)
	}
}
