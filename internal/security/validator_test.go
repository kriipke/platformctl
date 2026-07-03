package security

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidator(t *testing.T) {
	tests := []struct {
		name    string
		config  *SecurityConfig
		wantErr bool
	}{
		{
			name:    "nil config uses default",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "valid config",
			config:  DefaultSecurityConfig(),
			wantErr: false,
		},
		{
			name: "custom config",
			config: &SecurityConfig{
				MaxStringLength:     500,
				RequireHTTPS:        false,
				AllowPrivateIPs:     true,
				PasswordMinLength:   8,
				RequireSpecialChars: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, validator)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, validator)
				assert.NotNil(t, validator.config)
				assert.NotNil(t, validator.dnsNameRegex)
				assert.NotNil(t, validator.emailRegex)
				assert.NotNil(t, validator.semverRegex)
				assert.NotNil(t, validator.vaultPathRegex)
				assert.NotNil(t, validator.k8sNameRegex)
				assert.NotNil(t, validator.sqlInjectionRegex)
				assert.NotNil(t, validator.xssRegex)
				assert.NotNil(t, validator.commandInjRegex)
				assert.NotNil(t, validator.pathTraversalRegex)
			}
		})
	}
}

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	assert.Equal(t, 1000, config.MaxStringLength)
	assert.Equal(t, 1024*1024, config.MaxJSONSize)
	assert.Equal(t, []string{"http", "https", "git", "ssh"}, config.AllowedSchemes)
	assert.Equal(t, []string{}, config.BlockedDomains)
	assert.True(t, config.RequireHTTPS)
	assert.False(t, config.AllowPrivateIPs)
	assert.Equal(t, 12, config.PasswordMinLength)
	assert.Equal(t, 128, config.PasswordMaxLength)
	assert.True(t, config.RequireUppercase)
	assert.True(t, config.RequireLowercase)
	assert.True(t, config.RequireNumbers)
	assert.True(t, config.RequireSpecialChars)
}

func TestValidatorValidateString(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid string",
			input:     "Hello, World!",
			fieldName: "message",
			wantErr:   false,
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "message",
			wantErr:   true,
			errMsg:    "message is required",
		},
		{
			name:      "too long string",
			input:     strings.Repeat("a", 1001),
			fieldName: "message",
			wantErr:   true,
			errMsg:    "exceeds maximum length",
		},
		{
			name:      "invalid UTF-8",
			input:     string([]byte{0xff, 0xfe, 0xfd}),
			fieldName: "message",
			wantErr:   true,
			errMsg:    "invalid UTF-8",
		},
		{
			name:      "SQL injection attempt",
			input:     "'; DROP TABLE users; --",
			fieldName: "query",
			wantErr:   true,
			errMsg:    "potential SQL injection patterns",
		},
		{
			name:      "XSS attempt",
			input:     "<script>alert('xss')</script>",
			fieldName: "content",
			wantErr:   true,
			errMsg:    "potential XSS patterns",
		},
		{
			name:      "command injection attempt",
			input:     "file.txt; rm -rf /",
			fieldName: "filename",
			wantErr:   true,
			errMsg:    "potential command injection patterns",
		},
		{
			name:      "path traversal attempt",
			input:     "../../etc/passwd",
			fieldName: "path",
			wantErr:   true,
			errMsg:    "potential path traversal patterns",
		},
		{
			name:      "dangerous control characters",
			input:     "hello\x00world",
			fieldName: "message",
			wantErr:   true,
			errMsg:    "dangerous control characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateString(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateDNSName(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid DNS name",
			input:     "example-app",
			fieldName: "name",
			wantErr:   false,
		},
		{
			name:      "valid DNS name with numbers",
			input:     "app-123",
			fieldName: "name",
			wantErr:   false,
		},
		{
			name:      "single character",
			input:     "a",
			fieldName: "name",
			wantErr:   false,
		},
		{
			name:      "uppercase letters",
			input:     "Example-App",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "valid DNS-1123 label",
		},
		{
			name:      "starts with hyphen",
			input:     "-example",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "valid DNS-1123 label",
		},
		{
			name:      "ends with hyphen",
			input:     "example-",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "valid DNS-1123 label",
		},
		{
			name:      "contains underscore",
			input:     "example_app",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "valid DNS-1123 label",
		},
		{
			name:      "too long",
			input:     strings.Repeat("a", 64),
			fieldName: "name",
			wantErr:   true,
			errMsg:    "exceeds DNS label length limit",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateDNSName(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateEmail(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid email",
			input:     "user@example.com",
			fieldName: "email",
			wantErr:   false,
		},
		{
			name:      "valid email with subdomain",
			input:     "user@api.example.com",
			fieldName: "email",
			wantErr:   false,
		},
		{
			name:      "valid email with numbers",
			input:     "user123@example123.com",
			fieldName: "email",
			wantErr:   false,
		},
		{
			name:      "valid email with special chars",
			input:     "user.name+tag@example.com",
			fieldName: "email",
			wantErr:   false,
		},
		{
			name:      "missing @",
			input:     "userexample.com",
			fieldName: "email",
			wantErr:   true,
			errMsg:    "valid email address",
		},
		{
			name:      "missing domain",
			input:     "user@",
			fieldName: "email",
			wantErr:   true,
			errMsg:    "valid email address",
		},
		{
			name:      "missing local part",
			input:     "@example.com",
			fieldName: "email",
			wantErr:   true,
			errMsg:    "valid email address",
		},
		{
			name:      "invalid TLD",
			input:     "user@example.c",
			fieldName: "email",
			wantErr:   true,
			errMsg:    "valid email address",
		},
		{
			name:      "local part too long",
			input:     strings.Repeat("a", 65) + "@example.com",
			fieldName: "email",
			wantErr:   true,
			errMsg:    "local part exceeds 64 characters",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "email",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateEmail(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateURL(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid HTTPS URL",
			input:     "https://example.com/path",
			fieldName: "url",
			wantErr:   false,
		},
		{
			name:      "valid git URL",
			input:     "git://github.com/user/repo.git",
			fieldName: "url",
			wantErr:   false,
		},
		{
			name:      "valid SSH URL",
			input:     "ssh://git@github.com/user/repo.git",
			fieldName: "url",
			wantErr:   false,
		},
		{
			name:      "HTTP URL (rejected by default config)",
			input:     "http://example.com",
			fieldName: "url",
			wantErr:   true,
			errMsg:    "must use HTTPS",
		},
		{
			name:      "unsupported scheme",
			input:     "ftp://example.com",
			fieldName: "url",
			wantErr:   true,
			errMsg:    "scheme 'ftp' is not allowed",
		},
		{
			name:      "malformed URL",
			input:     "not-a-url",
			fieldName: "url",
			wantErr:   true,
			errMsg:    "not a valid URL",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "url",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateURL(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateURLWithCustomConfig(t *testing.T) {
	config := &SecurityConfig{
		MaxStringLength: 1000,
		AllowedSchemes:  []string{"http", "https"},
		BlockedDomains:  []string{"malicious.com", "blocked.org"},
		RequireHTTPS:    false,
		AllowPrivateIPs: true,
	}

	validator, err := NewValidator(config)
	require.NoError(t, err)

	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "HTTP allowed with custom config",
			input:   "http://example.com",
			wantErr: false,
		},
		{
			name:    "blocked domain",
			input:   "https://malicious.com/path",
			wantErr: true,
			errMsg:  "domain 'malicious.com' is blocked",
		},
		{
			name:    "subdomain of blocked domain",
			input:   "https://api.blocked.org/endpoint",
			wantErr: true,
			errMsg:  "domain 'blocked.org' is blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateURL(tt.input, "url")

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateUUID(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	validUUID := uuid.New().String()

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid UUID",
			input:     validUUID,
			fieldName: "id",
			wantErr:   false,
		},
		{
			name:      "valid nil UUID",
			input:     "00000000-0000-0000-0000-000000000000",
			fieldName: "id",
			wantErr:   false,
		},
		{
			name:      "invalid UUID format",
			input:     "not-a-uuid",
			fieldName: "id",
			wantErr:   true,
			errMsg:    "not a valid UUID",
		},
		{
			name:      "UUID without hyphens",
			input:     strings.Replace(validUUID, "-", "", -1),
			fieldName: "id",
			wantErr:   true,
			errMsg:    "not a valid UUID",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "id",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateUUID(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateSemVer(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid semantic version",
			input:     "1.0.0",
			fieldName: "version",
			wantErr:   false,
		},
		{
			name:      "valid version with v prefix",
			input:     "v2.1.3",
			fieldName: "version",
			wantErr:   false,
		},
		{
			name:      "valid version with pre-release",
			input:     "1.0.0-alpha.1",
			fieldName: "version",
			wantErr:   false,
		},
		{
			name:      "valid version with build metadata",
			input:     "1.0.0+build.123",
			fieldName: "version",
			wantErr:   false,
		},
		{
			name:      "valid version with pre-release and build",
			input:     "1.0.0-beta.2+build.456",
			fieldName: "version",
			wantErr:   false,
		},
		{
			name:      "invalid version missing patch",
			input:     "1.0",
			fieldName: "version",
			wantErr:   true,
			errMsg:    "not a valid semantic version",
		},
		{
			name:      "invalid version with leading zeros",
			input:     "01.0.0",
			fieldName: "version",
			wantErr:   true,
			errMsg:    "not a valid semantic version",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "version",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateSemVer(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateVaultPath(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid vault path",
			input:     "secret/data/app",
			fieldName: "path",
			wantErr:   false,
		},
		{
			name:      "valid path with hyphens and underscores",
			input:     "kv/app-data/user_secrets",
			fieldName: "path",
			wantErr:   false,
		},
		{
			name:      "path starting with slash",
			input:     "/secret/data",
			fieldName: "path",
			wantErr:   true,
			errMsg:    "cannot start with '/'",
		},
		{
			name:      "path without slash",
			input:     "secretdata",
			fieldName: "path",
			wantErr:   true,
			errMsg:    "must contain at least one '/'",
		},
		{
			name:      "path with invalid characters",
			input:     "secret/data@app",
			fieldName: "path",
			wantErr:   true,
			errMsg:    "contains invalid characters",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "path",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateVaultPath(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateKubernetesName(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid kubernetes name",
			input:     "my-app",
			fieldName: "name",
			wantErr:   false,
		},
		{
			name:      "valid name with dots",
			input:     "app.example.com",
			fieldName: "name",
			wantErr:   false,
		},
		{
			name:      "valid name with numbers",
			input:     "app-123.v2",
			fieldName: "name",
			wantErr:   false,
		},
		{
			name:      "name with uppercase",
			input:     "My-App",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "not a valid Kubernetes name",
		},
		{
			name:      "name starting with hyphen",
			input:     "-my-app",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "not a valid Kubernetes name",
		},
		{
			name:      "name ending with hyphen",
			input:     "my-app-",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "not a valid Kubernetes name",
		},
		{
			name:      "name too long",
			input:     strings.Repeat("a", 254),
			fieldName: "name",
			wantErr:   true,
			errMsg:    "exceeds Kubernetes name length limit",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "name",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateKubernetesName(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateIPAddress(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid IPv4 public",
			input:     "8.8.8.8",
			fieldName: "ip",
			wantErr:   false,
		},
		{
			name:      "valid IPv6 public",
			input:     "2001:db8::1",
			fieldName: "ip",
			wantErr:   false,
		},
		{
			name:      "private IPv4 (rejected by default)",
			input:     "192.168.1.1",
			fieldName: "ip",
			wantErr:   true,
			errMsg:    "private IP addresses are not allowed",
		},
		{
			name:      "loopback IPv4 (rejected by default)",
			input:     "127.0.0.1",
			fieldName: "ip",
			wantErr:   true,
			errMsg:    "private IP addresses are not allowed",
		},
		{
			name:      "private IPv6 (rejected by default)",
			input:     "fc00::1",
			fieldName: "ip",
			wantErr:   true,
			errMsg:    "private IP addresses are not allowed",
		},
		{
			name:      "invalid IP format",
			input:     "256.256.256.256",
			fieldName: "ip",
			wantErr:   true,
			errMsg:    "not a valid IP address",
		},
		{
			name:      "hostname not IP",
			input:     "example.com",
			fieldName: "ip",
			wantErr:   true,
			errMsg:    "not a valid IP address",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "ip",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateIPAddress(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateIPAddressWithAllowPrivate(t *testing.T) {
	config := &SecurityConfig{
		MaxStringLength: 1000,
		AllowPrivateIPs: true,
	}

	validator, err := NewValidator(config)
	require.NoError(t, err)

	// Private IPs should now be allowed
	err = validator.ValidateIPAddress("192.168.1.1", "ip")
	assert.NoError(t, err)

	err = validator.ValidateIPAddress("10.0.0.1", "ip")
	assert.NoError(t, err)

	err = validator.ValidateIPAddress("127.0.0.1", "ip")
	assert.NoError(t, err)
}

func TestValidatorValidatePassword(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid strong password",
			input:     "MyStr0ngP@ssw0rd!",
			fieldName: "password",
			wantErr:   false,
		},
		{
			name:      "minimum length strong password",
			input:     "MyP@ssw0rd12",
			fieldName: "password",
			wantErr:   false,
		},
		{
			name:      "too short",
			input:     "Short1!",
			fieldName: "password",
			wantErr:   true,
			errMsg:    "must be at least 12 characters long",
		},
		{
			name:      "too long",
			input:     strings.Repeat("a", 129) + "A1!",
			fieldName: "password",
			wantErr:   true,
			errMsg:    "exceeds maximum length",
		},
		{
			name:      "missing uppercase",
			input:     "mypassword123!",
			fieldName: "password",
			wantErr:   true,
			errMsg:    "must contain at least one uppercase letter",
		},
		{
			name:      "missing lowercase",
			input:     "MYPASSWORD123!",
			fieldName: "password",
			wantErr:   true,
			errMsg:    "must contain at least one lowercase letter",
		},
		{
			name:      "missing numbers",
			input:     "MyPassword!",
			fieldName: "password",
			wantErr:   true,
			errMsg:    "must contain at least one number",
		},
		{
			name:      "missing special characters",
			input:     "MyPassword123",
			fieldName: "password",
			wantErr:   true,
			errMsg:    "must contain at least one special character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePassword(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidatePasswordWithCustomConfig(t *testing.T) {
	config := &SecurityConfig{
		MaxStringLength:     1000,
		PasswordMinLength:   8,
		PasswordMaxLength:   64,
		RequireUppercase:    false,
		RequireLowercase:    true,
		RequireNumbers:      true,
		RequireSpecialChars: false,
	}

	validator, err := NewValidator(config)
	require.NoError(t, err)

	// Should pass with relaxed requirements
	err = validator.ValidatePassword("mypassword123", "password")
	assert.NoError(t, err)

	// Should still fail if missing required components
	err = validator.ValidatePassword("mypassword", "password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must contain at least one number")
}

func TestValidatorValidateBase64(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid base64",
			input:     "SGVsbG8gV29ybGQ=",
			fieldName: "data",
			wantErr:   false,
		},
		{
			name:      "valid base64 without padding",
			input:     "SGVsbG8",
			fieldName: "data",
			wantErr:   true,
			errMsg:    "not valid base64",
		},
		{
			name:      "invalid base64 characters",
			input:     "SGVsbG8gV29ybGQ@",
			fieldName: "data",
			wantErr:   true,
			errMsg:    "not valid base64",
		},
		{
			name:      "empty string",
			input:     "",
			fieldName: "data",
			wantErr:   true,
			errMsg:    "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateBase64(tt.input, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateJSONSize(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name      string
		data      []byte
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid small JSON",
			data:      []byte(`{"key": "value"}`),
			fieldName: "payload",
			wantErr:   false,
		},
		{
			name:      "JSON at size limit",
			data:      make([]byte, 1024*1024), // 1MB
			fieldName: "payload",
			wantErr:   false,
		},
		{
			name:      "JSON exceeds size limit",
			data:      make([]byte, 1024*1024+1), // 1MB + 1 byte
			fieldName: "payload",
			wantErr:   true,
			errMsg:    "exceeds maximum JSON size",
		},
		{
			name:      "empty JSON",
			data:      []byte{},
			fieldName: "payload",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateJSONSize(tt.data, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorValidateTimeRange(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	farFuture := now.Add(366 * 24 * time.Hour)

	tests := []struct {
		name      string
		start     time.Time
		end       time.Time
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid time range",
			start:     past,
			end:       future,
			fieldName: "timeRange",
			wantErr:   false,
		},
		{
			name:      "same start and end time",
			start:     now,
			end:       now,
			fieldName: "timeRange",
			wantErr:   false,
		},
		{
			name:      "end before start",
			start:     future,
			end:       past,
			fieldName: "timeRange",
			wantErr:   true,
			errMsg:    "end time must be after start time",
		},
		{
			name:      "range too long",
			start:     now,
			end:       farFuture,
			fieldName: "timeRange",
			wantErr:   true,
			errMsg:    "exceeds maximum allowed duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTimeRange(tt.start, tt.end, tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorSanitizeString(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean string",
			input:    "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "string with control characters",
			input:    "Hello\x00\x01World\x02",
			expected: "HelloWorld",
		},
		{
			name:     "string with newlines and tabs (preserved)",
			input:    "Hello\nWorld\t!",
			expected: "Hello\nWorld\t!",
		},
		{
			name:     "string too long",
			input:    strings.Repeat("a", 1500),
			expected: strings.Repeat("a", 1000),
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unicode characters",
			input:    "Hello 世界 🌍",
			expected: "Hello 世界 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.SanitizeString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatorSecurityThreatDetection(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	securityThreats := []struct {
		name   string
		input  string
		threat string
	}{
		{
			name:   "SQL injection - UNION",
			input:  "'; UNION SELECT * FROM users --",
			threat: "SQL injection",
		},
		{
			name:   "SQL injection - DROP",
			input:  "'; DROP TABLE users; --",
			threat: "SQL injection",
		},
		{
			name:   "XSS - script tag",
			input:  "<script>alert('xss')</script>",
			threat: "XSS",
		},
		{
			name:   "XSS - javascript protocol",
			input:  "javascript:alert('xss')",
			threat: "XSS",
		},
		{
			name:   "XSS - event handler",
			input:  "<img onerror=\"alert('xss')\" src=\"x\">",
			threat: "XSS",
		},
		{
			name:   "Command injection - semicolon rm",
			input:  "file.txt; rm -rf /",
			threat: "command injection",
		},
		{
			name:   "Command injection - pipe cat",
			input:  "input | cat /etc/passwd",
			threat: "command injection",
		},
		{
			name:   "Path traversal - dot dot slash",
			input:  "../../etc/passwd",
			threat: "path traversal",
		},
		{
			name:   "Path traversal - URL encoded",
			input:  "%2e%2e%2f%2e%2e%2fetc%2fpasswd",
			threat: "path traversal",
		},
	}

	for _, tt := range securityThreats {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateString(tt.input, "input")
			assert.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), tt.threat)
		})
	}
}

func TestValidatorPrivateIPDetection(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	privateIPs := []string{
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"127.0.0.1",
		"fc00::1",
		"::1",
	}

	for _, ip := range privateIPs {
		t.Run("private_ip_"+ip, func(t *testing.T) {
			err := validator.ValidateIPAddress(ip, "ip")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "private IP addresses are not allowed")
		})
	}

	publicIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"2001:db8::1",
	}

	for _, ip := range publicIPs {
		t.Run("public_ip_"+ip, func(t *testing.T) {
			err := validator.ValidateIPAddress(ip, "ip")
			assert.NoError(t, err)
		})
	}
}

// Benchmark tests
func BenchmarkValidatorValidateString(b *testing.B) {
	validator, _ := NewValidator(DefaultSecurityConfig())
	testString := "This is a test string for benchmarking validation performance"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = validator.ValidateString(testString, "test")
		}
	})
}

func BenchmarkValidatorValidateEmail(b *testing.B) {
	validator, _ := NewValidator(DefaultSecurityConfig())
	testEmail := "user@example.com"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = validator.ValidateEmail(testEmail, "email")
		}
	})
}

func BenchmarkValidatorValidateURL(b *testing.B) {
	validator, _ := NewValidator(DefaultSecurityConfig())
	testURL := "https://api.example.com/v1/endpoint"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = validator.ValidateURL(testURL, "url")
		}
	})
}

func BenchmarkValidatorValidateUUID(b *testing.B) {
	validator, _ := NewValidator(DefaultSecurityConfig())
	testUUID := uuid.New().String()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = validator.ValidateUUID(testUUID, "id")
		}
	})
}
