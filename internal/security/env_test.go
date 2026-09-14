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

func TestMaskEnvironment_NilAndEmpty(t *testing.T) {
	if got := MaskEnvironment(nil); got != nil {
		t.Errorf("MaskEnvironment(nil) = %v, want nil", got)
	}
	empty := map[string]string{}
	got := MaskEnvironment(empty)
	if got == nil || len(got) != 0 {
		t.Errorf("MaskEnvironment(empty) = %v, want empty map", got)
	}
}

func TestIsSensitive_CaseInsensitive(t *testing.T) {
	if !isSensitive("api_key") {
		t.Error("expected lowercase api_key to be sensitive")
	}
	if !isSensitive("API_KEY") {
		t.Error("expected uppercase API_KEY to be sensitive")
	}
	if !isSensitive("ApiKey") {
		t.Error("expected mixed-case ApiKey to be sensitive")
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

func TestIsSensitive_PreviouslyMissedCategories(t *testing.T) {
	sensitive := []string{
		"JWT", "SESSION_COOKIE", "TLS_CERT", "SLACK_WEBHOOK_URL",
		"SSN", "CC_NUMBER", "CREDIT_CARD",
	}
	for _, key := range sensitive {
		if !isSensitive(key) {
			t.Errorf("expected %s to be sensitive", key)
		}
	}
}

func TestMaskEnvironment_ValueShapeWithoutSensitiveName(t *testing.T) {
	env := map[string]string{
		"STRIPE_SK_LIVE": "sk_live_leaked123",
		"AWS_IDENTIFIER": "AKIAIOSFODNN7EXAMPLE",
		"NORMAL_VALUE":   "hello",
	}
	masked := MaskEnvironment(env)
	if masked["STRIPE_SK_LIVE"] != "********" {
		t.Errorf("STRIPE_SK_LIVE should be masked by value shape (sk_live_ prefix), got %s", masked["STRIPE_SK_LIVE"])
	}
	if masked["AWS_IDENTIFIER"] != "********" {
		t.Errorf("AWS_IDENTIFIER should be masked by value shape (AKIA prefix), got %s", masked["AWS_IDENTIFIER"])
	}
	if masked["NORMAL_VALUE"] != "hello" {
		t.Errorf("NORMAL_VALUE should not be masked, got %s", masked["NORMAL_VALUE"])
	}
}
