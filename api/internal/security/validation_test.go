package security

import (
	"testing"
)

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "Valid strong password",
			password: "MyP@ssw0rd123",
			wantErr:  false,
		},
		{
			name:     "Valid minimum requirements",
			password: "Abcd1234",
			wantErr:  false,
		},
		{
			name:     "Too short",
			password: "Abc123",
			wantErr:  true,
		},
		{
			name:     "No uppercase",
			password: "abcd1234",
			wantErr:  true,
		},
		{
			name:     "No lowercase",
			password: "ABCD1234",
			wantErr:  true,
		},
		{
			name:     "No numbers",
			password: "Abcdefgh",
			wantErr:  true,
		},
		{
			name:     "Empty password",
			password: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePasswordStrength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckPasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     PasswordStrength
	}{
		{
			name:     "Strong password with special chars",
			password: "MyP@ssw0rd!123",
			want:     PasswordStrong,
		},
		{
			name:     "Medium password",
			password: "MyPassword1",
			want:     PasswordMedium,
		},
		{
			name:     "Weak password",
			password: "pass",
			want:     PasswordWeak,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckPasswordStrength(tt.password)
			if got != tt.want {
				t.Errorf("CheckPasswordStrength() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "Valid email",
			email:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "Valid email with subdomain",
			email:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "Invalid - no @",
			email:   "userexample.com",
			wantErr: true,
		},
		{
			name:    "Invalid - no domain",
			email:   "user@",
			wantErr: true,
		},
		{
			name:    "Empty email",
			email:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "Valid username",
			username: "john_doe",
			wantErr:  false,
		},
		{
			name:     "Valid with numbers",
			username: "user123",
			wantErr:  false,
		},
		{
			name:     "Valid with hyphen",
			username: "john-doe",
			wantErr:  false,
		},
		{
			name:     "Too short",
			username: "ab",
			wantErr:  true,
		},
		{
			name:     "Invalid characters",
			username: "user@name",
			wantErr:  true,
		},
		{
			name:     "With spaces",
			username: "john doe",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "Normal filename",
			filename: "document.pdf",
			want:     "document.pdf",
		},
		{
			name:     "With dangerous characters",
			filename: "../../../etc/passwd",
			want:     ".._.._.._etc_passwd",
		},
		{
			name:     "With Windows path",
			filename: "C:\\Windows\\System32\\file.txt",
			want:     "C__Windows_System32_file.txt",
		},
		{
			name:     "With special characters",
			filename: "file<>:|?*.txt",
			want:     "file______.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.filename)
			if got != tt.want {
				t.Errorf("SanitizeFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAllowedFileType(t *testing.T) {
	allowedTypes := []string{"application/pdf", "image/jpeg", "image/png"}

	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{
			name:        "Allowed PDF",
			contentType: "application/pdf",
			want:        true,
		},
		{
			name:        "Allowed JPEG",
			contentType: "image/jpeg",
			want:        true,
		},
		{
			name:        "Not allowed",
			contentType: "application/javascript",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAllowedFileType(tt.contentType, allowedTypes)
			if got != tt.want {
				t.Errorf("IsAllowedFileType() = %v, want %v", got, tt.want)
			}
		})
	}
}
