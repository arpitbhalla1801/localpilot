package cli

import "testing"

func TestParsePort(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"3000", 3000, false},
		{"8080", 8080, false},
		{"1", 1, false},
		{"65535", 65535, false},
		{"0", 0, true},
		{"65536", 0, true},
		{"abc", 0, true},
		{"-1", 0, true},
	}

	for _, tt := range tests {
		got, err := parsePort(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parsePort(%s) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePort(%s) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parsePort(%s) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParsePID(t *testing.T) {
	tests := []struct {
		input   string
		want    int32
		wantErr bool
	}{
		{"1234", 1234, false},
		{"1", 1, false},
		{"0", 0, true},
		{"abc", 0, true},
		{"-1", 0, true},
	}

	for _, tt := range tests {
		got, err := parsePID(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parsePID(%s) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePID(%s) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parsePID(%s) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
