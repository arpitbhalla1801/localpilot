package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/arpitbhalla1801/localpilot/internal/agent"
	"github.com/arpitbhalla1801/localpilot/internal/models"
)

func TestDoctorCmd_JSON(t *testing.T) {
	if _, err := agent.New(); err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"doctor", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor --json failed: %v", err)
	}

	var got []models.Conflict
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON array: %v\noutput: %s", err, out.String())
	}
}

func TestDoctorCmd_Text(t *testing.T) {
	if _, err := agent.New(); err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"doctor"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
}
