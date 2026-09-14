package cli

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/localpilot/localpilot/internal/models"
)

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
		{"3000abc", 0, true},
		{"+80", 0, true},
		{"30 00", 0, true},
		{" 3000", 0, true},
		{"3000 ", 0, true},
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
		{"1234xyz", 0, true},
		{"+1234", 0, true},
		{"2147483647", 2147483647, false}, // math.MaxInt32
		{"2147483648", 0, true},           // math.MaxInt32 + 1
		{"4294967296", 0, true},           // 2^32, wraps to 0 as int32 if unchecked
		{"4294967297", 0, true},           // 2^32 + 1, wraps to 1 as int32 if unchecked
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

func TestInsertDashDashForNegativeArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"inspect negative", []string{"inspect", "-5"}, []string{"inspect", "--", "-5"}},
		{"port negative", []string{"port", "-5"}, []string{"port", "--", "-5"}},
		{"kill negative", []string{"kill", "-5"}, []string{"kill", "--", "-5"}},
		{"positive PID untouched", []string{"inspect", "5"}, []string{"inspect", "5"}},
		{"already has --", []string{"inspect", "--", "-5"}, []string{"inspect", "--", "-5"}},
		{"other flag untouched", []string{"kill", "--force"}, []string{"kill", "--force"}},
		{"unrelated command untouched", []string{"list", "-5"}, []string{"list", "-5"}},
		{"list --all unaffected", []string{"list", "--all"}, []string{"list", "--all"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := insertDashDashForNegativeArgs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("insertDashDashForNegativeArgs(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNonNilPorts(t *testing.T) {
	if got := nonNilPorts(nil); got == nil {
		t.Error("nonNilPorts(nil) returned nil, want an empty non-nil slice")
	}

	b, err := json.Marshal(nonNilPorts(nil))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("json.Marshal(nonNilPorts(nil)) = %s, want []", b)
	}

	populated := []models.Port{{Number: 80}}
	if got := nonNilPorts(populated); len(got) != 1 {
		t.Errorf("nonNilPorts(populated) = %v, want unchanged populated slice", got)
	}
}
