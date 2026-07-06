package security

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrInputTooLong        = errors.New("input too long")
	ErrInvalidFormat       = errors.New("invalid format")
	ErrSuspiciousInput     = errors.New("suspicious input detected")
	ErrDangerousCharacters = errors.New("dangerous characters detected")
	ErrInvalidUUID         = errors.New("invalid UUID format")
	ErrInvalidEmail        = errors.New("invalid email format")
	ErrInvalidURL          = errors.New("invalid URL format")
	ErrInvalidIPAddress    = errors.New("invalid IP address")
	ErrWeakPassword        = errors.New("password does not meet security requirements")
)

// SecurityConfig holds configuration for security validation
type SecurityConfig struct {
	MaxStringLength     int      `json:"max_string_length"`
	MaxJSONSize         int      `json:"max_json_size"`
	AllowedSchemes      []string `json:"allowed_schemes"`
	BlockedDomains      []string `json:"blocked_domains"`
	RequireHTTPS        bool     `json:"require_https"`
	AllowPrivateIPs     bool     `json:"allow_private_ips"`
	PasswordMinLength   int      `json:"password_min_length"`
	PasswordMaxLength   int      `json:"password_max_length"`
	RequireUppercase    bool     `json:"require_uppercase"`
	RequireLowercase    bool     `json:"require_lowercase"`
	RequireNumbers      bool     `json:"require_numbers"`
	RequireSpecialChars bool     `json:"require_special_chars"`
}

// DefaultSecurityConfig returns a secure default configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxStringLength:     1000,
		MaxJSONSize:         1024 * 1024, // 1MB
		AllowedSchemes:      []string{"http", "https", "git", "ssh"},
		BlockedDomains:      []string{},
		RequireHTTPS:        true,
		AllowPrivateIPs:     false,
		PasswordMinLength:   12,
		PasswordMaxLength:   128,
		RequireUppercase:    true,
		RequireLowercase:    true,
		RequireNumbers:      true,
		RequireSpecialChars: true,
	}
}

// Validator provides security-focused input validation
type Validator struct {
	config *SecurityConfig

	// Compiled regular expressions for performance
	dnsNameRegex       *regexp.Regexp
	emailRegex         *regexp.Regexp
	semverRegex        *regexp.Regexp
	vaultPathRegex     *regexp.Regexp
	k8sNameRegex       *regexp.Regexp
	sqlInjectionRegex  *regexp.Regexp
	xssRegex           *regexp.Regexp
	commandInjRegex    *regexp.Regexp
	pathTraversalRegex *regexp.Regexp
}

// NewValidator creates a new security validator
func NewValidator(config *SecurityConfig) (*Validator, error) {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	v := &Validator{config: config}

	// Compile regular expressions
	var err error

	// DNS-1123 compliant names (RFC 1123)
	v.dnsNameRegex, err = regexp.Compile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile DNS name regex: %w", err)
	}

	// Email validation (RFC 5322 compliant)
	v.emailRegex, err = regexp.Compile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile email regex: %w", err)
	}

	// Semantic versioning (semver)
	v.semverRegex, err = regexp.Compile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile semver regex: %w", err)
	}

	// Vault path validation
	v.vaultPathRegex, err = regexp.Compile(`^[a-zA-Z0-9/_-]+$`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile vault path regex: %w", err)
	}

	// Kubernetes name validation
	v.k8sNameRegex, err = regexp.Compile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile k8s name regex: %w", err)
	}

	// Security threat detection patterns.
	// script:/javascript:/vbscript: are XSS/script-injection vectors and live in
	// the XSS pattern below, not here — otherwise they'd be matched first and
	// mislabelled as SQL injection.
	v.sqlInjectionRegex, err = regexp.Compile(`(?i)(union\s+(all\s+)?select|insert\s+into|delete\s+from|update\s+.+\s+set|drop\s+(table|database)|exec\s*\()`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile SQL injection regex: %w", err)
	}

	v.xssRegex, err = regexp.Compile(`(?i)(<script[^>]*>.*?</script>|script\s*:|javascript\s*:|vbscript\s*:|onload\s*=|onerror\s*=|onclick\s*=|onmouseover\s*=)`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile XSS regex: %w", err)
	}

	v.commandInjRegex, err = regexp.Compile(`(?i)(;\s*rm\s|;\s*cat\s|;\s*ls\s|;\s*wget\s|;\s*curl\s|;\s*nc\s|;\s*bash\s|;\s*sh\s|\|\s*rm\s|\|\s*cat\s|\|\s*ls\s|&&\s*rm\s|&&\s*cat\s)`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile command injection regex: %w", err)
	}

	v.pathTraversalRegex, err = regexp.Compile(`(\.\.\/|\.\.\\|%2e%2e%2f|%2e%2e%5c)`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile path traversal regex: %w", err)
	}

	return v, nil
}

// ValidateString performs comprehensive string validation
func (v *Validator) ValidateString(input string, fieldName string) error {
	if len(input) == 0 {
		return fmt.Errorf("%w: %s is required", ErrInvalidInput, fieldName)
	}

	if len(input) > v.config.MaxStringLength {
		return fmt.Errorf("%w: %s exceeds maximum length of %d", ErrInputTooLong, fieldName, v.config.MaxStringLength)
	}

	// Check for valid UTF-8
	if !utf8.ValidString(input) {
		return fmt.Errorf("%w: %s contains invalid UTF-8", ErrInvalidFormat, fieldName)
	}

	// Check for security threats
	if err := v.checkSecurityThreats(input, fieldName); err != nil {
		return err
	}

	// Check for dangerous characters
	if err := v.checkDangerousCharacters(input, fieldName); err != nil {
		return err
	}

	return nil
}

// ValidateDNSName validates DNS-compliant names
func (v *Validator) ValidateDNSName(name string, fieldName string) error {
	if err := v.ValidateString(name, fieldName); err != nil {
		return err
	}

	if len(name) > 63 {
		return fmt.Errorf("%w: %s exceeds DNS label length limit (63 characters)", ErrInputTooLong, fieldName)
	}

	if !v.dnsNameRegex.MatchString(name) {
		return fmt.Errorf("%w: %s must be a valid DNS-1123 label", ErrInvalidFormat, fieldName)
	}

	return nil
}

// ValidateEmail validates email addresses
func (v *Validator) ValidateEmail(email string, fieldName string) error {
	if err := v.ValidateString(email, fieldName); err != nil {
		return err
	}

	if !v.emailRegex.MatchString(email) {
		return fmt.Errorf("%w: %s must be a valid email address", ErrInvalidEmail, fieldName)
	}

	// Additional email security checks
	localPart := strings.Split(email, "@")[0]
	if len(localPart) > 64 {
		return fmt.Errorf("%w: %s local part exceeds 64 characters", ErrInvalidEmail, fieldName)
	}

	return nil
}

// ValidateURL validates URLs with security checks
func (v *Validator) ValidateURL(urlStr string, fieldName string) error {
	if err := v.ValidateString(urlStr, fieldName); err != nil {
		return err
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %s is not a valid URL: %v", ErrInvalidURL, fieldName, err)
	}

	// url.Parse is lenient and happily accepts relative/opaque strings such as
	// "not-a-url" (scheme and host both empty). Reject those explicitly as
	// malformed rather than letting them fall through to the scheme check with a
	// confusing "scheme '' is not allowed" message.
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("%w: %s is not a valid URL", ErrInvalidURL, fieldName)
	}

	// Check allowed schemes
	if len(v.config.AllowedSchemes) > 0 {
		allowed := false
		for _, scheme := range v.config.AllowedSchemes {
			if parsedURL.Scheme == scheme {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: %s scheme '%s' is not allowed", ErrInvalidURL, fieldName, parsedURL.Scheme)
		}
	}

	// Check HTTPS requirement for HTTP URLs
	if v.config.RequireHTTPS && parsedURL.Scheme == "http" {
		return fmt.Errorf("%w: %s must use HTTPS", ErrInvalidURL, fieldName)
	}

	// Check blocked domains
	for _, blocked := range v.config.BlockedDomains {
		if strings.Contains(parsedURL.Host, blocked) {
			return fmt.Errorf("%w: %s domain '%s' is blocked", ErrInvalidURL, fieldName, blocked)
		}
	}

	// Check for private IP addresses if not allowed
	if !v.config.AllowPrivateIPs {
		if err := v.checkPrivateIP(parsedURL.Host, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// ValidateUUID validates UUID format
func (v *Validator) ValidateUUID(uuidStr string, fieldName string) error {
	if err := v.ValidateString(uuidStr, fieldName); err != nil {
		return err
	}

	// uuid.Parse is lenient — it also accepts non-canonical forms such as a
	// hyphen-less 32-char string, a urn:uuid: prefix, or brace-wrapped values.
	// Require the canonical 8-4-4-4-12 hyphenated form by round-tripping through
	// String() (which always emits canonical lowercase) and comparing.
	parsed, err := uuid.Parse(uuidStr)
	if err != nil || parsed.String() != strings.ToLower(uuidStr) {
		return fmt.Errorf("%w: %s is not a valid UUID", ErrInvalidUUID, fieldName)
	}

	return nil
}

// ValidateSemVer validates semantic version strings
func (v *Validator) ValidateSemVer(version string, fieldName string) error {
	if err := v.ValidateString(version, fieldName); err != nil {
		return err
	}

	if !v.semverRegex.MatchString(version) {
		return fmt.Errorf("%w: %s is not a valid semantic version", ErrInvalidFormat, fieldName)
	}

	return nil
}

// ValidateVaultPath validates HashiCorp Vault paths
func (v *Validator) ValidateVaultPath(path string, fieldName string) error {
	if err := v.ValidateString(path, fieldName); err != nil {
		return err
	}

	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("%w: %s cannot start with '/'", ErrInvalidFormat, fieldName)
	}

	if !strings.Contains(path, "/") {
		return fmt.Errorf("%w: %s must contain at least one '/'", ErrInvalidFormat, fieldName)
	}

	if !v.vaultPathRegex.MatchString(path) {
		return fmt.Errorf("%w: %s contains invalid characters", ErrInvalidFormat, fieldName)
	}

	return nil
}

// ValidateKubernetesName validates Kubernetes resource names
func (v *Validator) ValidateKubernetesName(name string, fieldName string) error {
	if err := v.ValidateString(name, fieldName); err != nil {
		return err
	}

	if len(name) > 253 {
		return fmt.Errorf("%w: %s exceeds Kubernetes name length limit (253 characters)", ErrInputTooLong, fieldName)
	}

	if !v.k8sNameRegex.MatchString(name) {
		return fmt.Errorf("%w: %s is not a valid Kubernetes name", ErrInvalidFormat, fieldName)
	}

	return nil
}

// ValidateIPAddress validates IP addresses
func (v *Validator) ValidateIPAddress(ipStr string, fieldName string) error {
	if err := v.ValidateString(ipStr, fieldName); err != nil {
		return err
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return fmt.Errorf("%w: %s is not a valid IP address", ErrInvalidIPAddress, fieldName)
	}

	// Check private IP restrictions
	if !v.config.AllowPrivateIPs && v.isPrivateIP(ip) {
		return fmt.Errorf("%w: %s private IP addresses are not allowed", ErrInvalidIPAddress, fieldName)
	}

	return nil
}

// ValidatePassword validates password strength
func (v *Validator) ValidatePassword(password string, fieldName string) error {
	if len(password) < v.config.PasswordMinLength {
		return fmt.Errorf("%w: %s must be at least %d characters long", ErrWeakPassword, fieldName, v.config.PasswordMinLength)
	}

	if len(password) > v.config.PasswordMaxLength {
		return fmt.Errorf("%w: %s exceeds maximum length of %d characters", ErrWeakPassword, fieldName, v.config.PasswordMaxLength)
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasDigit   = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if v.config.RequireUppercase && !hasUpper {
		return fmt.Errorf("%w: %s must contain at least one uppercase letter", ErrWeakPassword, fieldName)
	}

	if v.config.RequireLowercase && !hasLower {
		return fmt.Errorf("%w: %s must contain at least one lowercase letter", ErrWeakPassword, fieldName)
	}

	if v.config.RequireNumbers && !hasDigit {
		return fmt.Errorf("%w: %s must contain at least one number", ErrWeakPassword, fieldName)
	}

	if v.config.RequireSpecialChars && !hasSpecial {
		return fmt.Errorf("%w: %s must contain at least one special character", ErrWeakPassword, fieldName)
	}

	return nil
}

// ValidateBase64 validates base64 encoded strings
func (v *Validator) ValidateBase64(encoded string, fieldName string) error {
	if err := v.ValidateString(encoded, fieldName); err != nil {
		return err
	}

	_, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("%w: %s is not valid base64", ErrInvalidFormat, fieldName)
	}

	return nil
}

// ValidateJSONSize validates JSON payload size
func (v *Validator) ValidateJSONSize(data []byte, fieldName string) error {
	if len(data) > v.config.MaxJSONSize {
		return fmt.Errorf("%w: %s exceeds maximum JSON size of %d bytes", ErrInputTooLong, fieldName, v.config.MaxJSONSize)
	}

	return nil
}

// ValidateTimeRange validates time range inputs
func (v *Validator) ValidateTimeRange(start, end time.Time, fieldName string) error {
	if end.Before(start) {
		return fmt.Errorf("%w: %s end time must be after start time", ErrInvalidInput, fieldName)
	}

	// Prevent extremely long time ranges (more than 1 year)
	if end.Sub(start) > 365*24*time.Hour {
		return fmt.Errorf("%w: %s time range exceeds maximum allowed duration", ErrInvalidInput, fieldName)
	}

	return nil
}

// Security threat detection methods

// checkSecurityThreats checks for common security attack patterns
func (v *Validator) checkSecurityThreats(input string, fieldName string) error {
	lowerInput := strings.ToLower(input)

	if v.sqlInjectionRegex.MatchString(lowerInput) {
		return fmt.Errorf("%w: %s contains potential SQL injection patterns", ErrSuspiciousInput, fieldName)
	}

	if v.xssRegex.MatchString(lowerInput) {
		return fmt.Errorf("%w: %s contains potential XSS patterns", ErrSuspiciousInput, fieldName)
	}

	if v.commandInjRegex.MatchString(lowerInput) {
		return fmt.Errorf("%w: %s contains potential command injection patterns", ErrSuspiciousInput, fieldName)
	}

	if v.pathTraversalRegex.MatchString(lowerInput) {
		return fmt.Errorf("%w: %s contains potential path traversal patterns", ErrSuspiciousInput, fieldName)
	}

	return nil
}

// checkDangerousCharacters checks for dangerous or suspicious characters
func (v *Validator) checkDangerousCharacters(input string, fieldName string) error {
	dangerousChars := []rune{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0B, 0x0C, 0x0E, 0x0F}

	for _, char := range input {
		for _, dangerous := range dangerousChars {
			if char == dangerous {
				return fmt.Errorf("%w: %s contains dangerous control characters", ErrDangerousCharacters, fieldName)
			}
		}
	}

	return nil
}

// checkPrivateIP checks if a hostname resolves to a private IP
func (v *Validator) checkPrivateIP(hostname string, fieldName string) error {
	// Try to resolve the hostname
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// If we can't resolve it, allow it (could be internal hostname)
		return nil
	}

	for _, ip := range ips {
		if v.isPrivateIP(ip) {
			return fmt.Errorf("%w: %s resolves to private IP address", ErrInvalidURL, fieldName)
		}
	}

	return nil
}

// isPrivateIP checks if an IP address is private
func (v *Validator) isPrivateIP(ip net.IP) bool {
	// IPv4 private ranges
	privateIPv4Ranges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8", // Loopback
	}

	// IPv6 private ranges
	privateIPv6Ranges := []string{
		"fc00::/7", // Unique local address
		"::1/128",  // Loopback
	}

	allRanges := append(privateIPv4Ranges, privateIPv6Ranges...)

	for _, cidr := range allRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// SanitizeString removes potentially dangerous characters and patterns
func (v *Validator) SanitizeString(input string) string {
	// Remove control characters except newline and tab
	result := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, input)

	// Limit length if necessary
	if len(result) > v.config.MaxStringLength {
		result = result[:v.config.MaxStringLength]
	}

	return result
}

// ValidateStruct validates a struct using field validation tags
func (v *Validator) ValidateStruct(s interface{}) error {
	// This would integrate with a struct validation library like go-playground/validator
	// For now, return nil as a placeholder
	return nil
}
