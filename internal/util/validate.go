package util

import (
	"fmt"
	"net"
	"net/mail"
	"path/filepath"
	"strings"
)

// ValidateEmail checks that the email address is syntactically valid.
func ValidateEmail(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return fmt.Errorf("email is required")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return fmt.Errorf("invalid email")
	}
	return nil
}

// ValidatePath rejects paths containing directory traversal sequences or
// leading tilde characters that could cause home-directory expansion ambiguity.
func ValidatePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	// Check the raw input for traversal patterns before filepath.Clean resolves them.
	if strings.Contains(path, "..") {
		return fmt.Errorf("path must not contain directory traversal (..)")
	}
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path must not contain directory traversal (..)")
	}
	if strings.HasPrefix(cleaned, "~") {
		return fmt.Errorf("path must not start with ~")
	}
	return nil
}

// ValidateCron performs basic syntax validation on a 5-field cron expression.
func ValidateCron(expr string) error {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return fmt.Errorf("cron expression must have exactly 5 fields")
	}
	return nil
}

// ValidateSSHHost rejects host values that are empty, contain whitespace, or
// contain shell-dangerous characters.
func ValidateSSHHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("host is required")
	}
	if strings.ContainsAny(host, " \t\n\r;|&$`\\\"'(){}[]") {
		return fmt.Errorf("host contains invalid characters")
	}
	if h, _, err := net.SplitHostPort(host); err == nil && h != "" {
		return fmt.Errorf("host must not include a port")
	}
	return nil
}

// ValidatePort checks that the port number is within the valid TCP range.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// ValidatePassword checks that the password meets the minimum length requirement.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}
