package security

import (
	"strings"
)

var sensitivePatterns = []string{
	"key", "secret", "password", "passwd", "token", "credential",
	"auth", "private", "api_key", "apikey", "access_key",
	"database", "db_url", "connection_string",
	"jwt", "cookie", "session", "cert", "webhook",
	"ssn", "social_security", "cc_number", "credit_card", "pin_code",
}

// sensitiveValuePrefixes catches secrets whose variable name gives no hint
// (e.g. Stripe's own naming convention doesn't contain "key" or "secret")
// but whose value has a recognizable, vendor-specific shape.
var sensitiveValuePrefixes = []string{
	"sk_live_", "sk_test_", // Stripe secret keys
	"AKIA", // AWS access key IDs
}

// MaskEnvironment redacts sensitive environment variable values.
func MaskEnvironment(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}

	masked := make(map[string]string, len(env))
	for key, value := range env {
		if isSensitive(key) || hasSensitiveValueShape(value) {
			masked[key] = "********"
		} else {
			masked[key] = value
		}
	}
	return masked
}

func isSensitive(key string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func hasSensitiveValueShape(value string) bool {
	for _, prefix := range sensitiveValuePrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
