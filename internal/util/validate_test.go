package util

import "testing"

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"user@example.com", true},
		{"USER@Example.COM", true},
		{"a@b.c", true},
		{"", false},
		{"  ", false},
		{"notanemail", false},
		{"@missing.local", false},
		{"missing@", false},
		{"has space@example.com", false},
	}
	for _, tt := range tests {
		err := ValidateEmail(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ValidateEmail(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidateEmail(%q) expected error", tt.input)
		}
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"/var/backups", true},
		{"/data/backup/files", true},
		{"relative/path", true},
		{"", false},
		{"  ", false},
		{"/var/../etc/passwd", false},
		{"../../../etc/shadow", false},
		{"/safe/dir/..\\windows", false},
		{"~/Desktop", false},
		{"~user/data", false},
	}
	for _, tt := range tests {
		err := ValidatePath(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ValidatePath(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidatePath(%q) expected error", tt.input)
		}
	}
}

func TestValidateCron(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"0 2 * * *", true},
		{"*/5 * * * *", true},
		{"", false},
		{"0 2 * *", false},
		{"0 2 * * * *", false},
	}
	for _, tt := range tests {
		err := ValidateCron(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ValidateCron(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidateCron(%q) expected error", tt.input)
		}
	}
}

func TestValidateSSHHost(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"192.168.1.1", true},
		{"example.com", true},
		{"backup-host.local", true},
		{"", false},
		{" ", false},
		{"host;rm -rf /", false},
		{"host|cat /etc/passwd", false},
		{"host&whoami", false},
		{"host$(id)", false},
		{"host`id`", false},
		{"host with space", false},
	}
	for _, tt := range tests {
		err := ValidateSSHHost(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ValidateSSHHost(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidateSSHHost(%q) expected error", tt.input)
		}
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		input int
		ok    bool
	}{
		{22, true},
		{1, true},
		{65535, true},
		{0, false},
		{-1, false},
		{65536, false},
	}
	for _, tt := range tests {
		err := ValidatePort(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ValidatePort(%d) unexpected error: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidatePort(%d) expected error", tt.input)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"12345678", true},
		{"strongpassword!", true},
		{"", false},
		{"short", false},
		{"1234567", false},
	}
	for _, tt := range tests {
		err := ValidatePassword(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ValidatePassword(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidatePassword(%q) expected error", tt.input)
		}
	}
}
