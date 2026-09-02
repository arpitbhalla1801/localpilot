package security

import (
	"strings"
)

var sensitivePatterns = []string{
	"key", "secret", "password", "passwd", "token", "credential",
	"auth", "private", "api_key", "apikey", "access_key",
	"database", "db_url", "connection_string",
}

// MaskEnvironment redacts sensitive environment variable values.
func MaskEnvironment(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}

	masked := make(map[string]string, len(env))
	for key, value := range env {
		if isSensitive(key) {
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
