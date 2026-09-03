package security

import "testing"

func TestMaskEnvironment(t *testing.T) {
	env := map[string]string{
		"HOME":           "/home/user",
		"DATABASE_URL":   "postgres://secret",
		"API_KEY":        "sk-12345",
		"NODE_ENV":       "development",
		"AWS_SECRET_KEY": "aws-secret",
	}

	masked := MaskEnvironment(env)

	if masked["HOME"] != "/home/user" {
		t.Errorf("HOME should not be masked, got %s", masked["HOME"])
	}
	if masked["NODE_ENV"] != "development" {
		t.Errorf("NODE_ENV should not be masked, got %s", masked["NODE_ENV"])
	}
	if masked["DATABASE_URL"] != "********" {
		t.Errorf("DATABASE_URL should be masked")
	}
	if masked["API_KEY"] != "********" {
		t.Errorf("API_KEY should be masked")
	}
	if masked["AWS_SECRET_KEY"] != "********" {
		t.Errorf("AWS_SECRET_KEY should be masked")
	}
}

func TestIsSensitive(t *testing.T) {
	sensitive := []string{"API_KEY", "my_password", "SECRET_TOKEN", "auth_header"}
	for _, key := range sensitive {
		if !isSensitive(key) {
			t.Errorf("expected %s to be sensitive", key)
		}
	}

	nonSensitive := []string{"HOME", "PATH", "NODE_ENV", "PORT"}
	for _, key := range nonSensitive {
		if isSensitive(key) {
			t.Errorf("expected %s to not be sensitive", key)
		}
	}
}
