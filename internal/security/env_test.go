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

// TestHasSensitiveValueShape_CaseInsensitive is the regression test for
// #39: the value-shape check used to be case-sensitive, so a Stripe key
// or AWS key ID whose case had been altered (e.g. by a shell or wrapper
// that uppercases exported values) slipped through unredacted — the only
// safety net for these, since their variable *names* don't reliably
// contain a sensitive-looking word like "key" or "secret".
func TestHasSensitiveValueShape_CaseInsensitive(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"sk_live_abc123", true},
		{"SK_LIVE_abc123", true},
		{"Sk_Live_abc123", true},
		{"sk_test_abc123", true},
		{"SK_TEST_ABC123", true},
		{"AKIAIOSFODNN7EXAMPLE", true},
		{"akiaiosfodnn7example", true},
		{"not-a-secret", false},
	}
	for _, tt := range tests {
		if got := hasSensitiveValueShape(tt.value); got != tt.want {
			t.Errorf("hasSensitiveValueShape(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestMaskEnvironment_ValueShapeCaseVariants(t *testing.T) {
	// Variable names deliberately avoid every word in sensitivePatterns
	// (key, secret, token, id is fine, etc.) so these can only be caught
	// by hasSensitiveValueShape, isolating the case-sensitivity fix from
	// isSensitive's separate, already-case-insensitive name matching.
	env := map[string]string{
		"X_STRIPE_ID": "SK_LIVE_leaked123",
		"X_AWS_ID":    "akiaiosfodnn7example",
	}
	masked := MaskEnvironment(env)
	if masked["X_STRIPE_ID"] != "********" {
		t.Errorf("X_STRIPE_ID with uppercase SK_LIVE_ value should be masked, got %s", masked["X_STRIPE_ID"])
	}
	if masked["X_AWS_ID"] != "********" {
		t.Errorf("X_AWS_ID with lowercase akia value should be masked, got %s", masked["X_AWS_ID"])
	}
}
