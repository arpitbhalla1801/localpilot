package platform

import (
	"reflect"
	"testing"
)

func TestContainerFromPSEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry dockerPSEntry
		want  string
	}{
		{
			name:  "leading slash is stripped",
			entry: dockerPSEntry{Names: "/my-app"},
			want:  "my-app",
		},
		{
			name:  "first of multiple aliases is used",
			entry: dockerPSEntry{Names: "my-app,my-app-alias"},
			want:  "my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerFromPSEntry(tt.entry)
			if got.Name != tt.want {
				t.Errorf("Name = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

func TestParseDockerHostPorts(t *testing.T) {
	tests := []struct {
		name  string
		ports string
		want  []int
	}{
		{
			name:  "empty",
			ports: "",
			want:  nil,
		},
		{
			name:  "single tcp mapping",
			ports: "0.0.0.0:3000->3000/tcp",
			want:  []int{3000},
		},
		{
			name:  "ipv4 and ipv6 mapping for the same port",
			ports: "0.0.0.0:3000->3000/tcp, :::3000->3000/tcp",
			want:  []int{3000, 3000},
		},
		{
			name:  "different host and container ports",
			ports: "0.0.0.0:8080->80/tcp",
			want:  []int{8080},
		},
		{
			name:  "exposed but unpublished port is skipped",
			ports: "8080/tcp",
			want:  nil,
		},
		{
			name:  "mixed published and unpublished ports",
			ports: "0.0.0.0:5432->5432/tcp, 9999/tcp",
			want:  []int{5432},
		},
		{
			name:  "multiple distinct published ports",
			ports: "0.0.0.0:3000->3000/tcp, 0.0.0.0:3001->3001/tcp",
			want:  []int{3000, 3001},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDockerHostPorts(tt.ports)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDockerHostPorts(%q) = %v, want %v", tt.ports, got, tt.want)
			}
		})
	}
}
