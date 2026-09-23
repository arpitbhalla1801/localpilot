package platform

import (
	"reflect"
	"testing"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// utf16leBytes encodes s the way wsl.exe emits its own output when piped
// (not a real console): UTF-16LE, no BOM. ASCII-only input is sufficient
// here since that's the format under test.
func utf16leBytes(s string) []byte {
	b := make([]byte, 0, len(s)*2)
	for _, r := range s {
		b = append(b, byte(r), 0)
	}
	return b
}

func TestParseWSLDistroList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"single distro", "archlinux\r\n", []string{"archlinux"}},
		{"multiple distros", "Ubuntu\r\narchlinux\r\n", []string{"Ubuntu", "archlinux"}},
		{"empty (no running distros)", "", nil},
		{"blank lines skipped", "archlinux\r\n\r\n", []string{"archlinux"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWSLDistroList(utf16leBytes(tt.in))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseWSLDistroList(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseSSListeners(t *testing.T) {
	// Real `ss -tlnp` output captured from a running WSL distro, header
	// included (must be skipped) plus a line with no resolvable process
	// (no users: column — e.g. permission denied for a foreign socket).
	out := `State  Recv-Q Send-Q      Local Address:Port  Peer Address:PortProcess
LISTEN 0      511             127.0.0.1:45031      0.0.0.0:*    users:(("node",pid=110,fd=31))
LISTEN 0      1000       10.255.255.254:53         0.0.0.0:*
LISTEN 0      50     [::ffff:127.0.0.1]:42593            *:*    users:(("java",pid=258557,fd=261))
`
	got := parseSSListeners("archlinux", []byte(out))

	want := map[int]*models.WSLProcess{
		45031: {Distro: "archlinux", PID: 110, Name: "node"},
		42593: {Distro: "archlinux", PID: 258557, Name: "java"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSSListeners() = %+v, want %+v", got, want)
	}
}

func TestParseSSListeners_NoListeners(t *testing.T) {
	got := parseSSListeners("archlinux", []byte("State  Recv-Q Send-Q      Local Address:Port  Peer Address:Port\n"))
	if len(got) != 0 {
		t.Errorf("parseSSListeners(no listeners) = %+v, want empty", got)
	}
}
