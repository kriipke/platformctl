package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// Built-in defaults, also used as the displayed flag defaults.
const (
	defaultServer   = "http://localhost:8080"
	defaultOutput   = "table"
	defaultUsername = "admin"
	defaultPassword = "admin"
)

// fileConfig mirrors the on-disk config file ($HOME/.platformctl.yaml). Keys
// use camelCase; sigs.k8s.io/yaml decodes YAML through the json tags.
type fileConfig struct {
	Server     string `json:"server,omitempty"`
	Output     string `json:"output,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	CustomerID string `json:"customerId,omitempty"`
	Token      string `json:"token,omitempty"`
	Insecure   bool   `json:"insecure,omitempty"`
}

// resolve merges flags, environment variables, the config file and defaults
// into the GlobalFlags struct, in that order of precedence. It is invoked from
// the root command's PersistentPreRunE so every subcommand sees final values.
func (f *GlobalFlags) resolve(cmd *cobra.Command) error {
	fc, err := loadFileConfig(f.ConfigFile)
	if err != nil {
		return err
	}

	f.Server = resolveStr(cmd, "server", f.Server, os.Getenv("PLATFORMCTL_SERVER"), fc.Server, defaultServer)
	f.Output = resolveStr(cmd, "output", f.Output, os.Getenv("PLATFORMCTL_OUTPUT"), fc.Output, defaultOutput)
	f.Username = resolveStr(cmd, "username", f.Username, os.Getenv("PLATFORMCTL_USERNAME"), fc.Username, defaultUsername)
	f.Password = resolveStr(cmd, "password", f.Password, os.Getenv("PLATFORMCTL_PASSWORD"), fc.Password, defaultPassword)
	f.CustomerID = resolveStr(cmd, "customer-id", f.CustomerID, os.Getenv("PLATFORMCTL_CUSTOMER_ID"), fc.CustomerID, "")
	f.Token = resolveStr(cmd, "token", f.Token, os.Getenv("PLATFORMCTL_TOKEN"), fc.Token, "")

	insecure, err := resolveBool(cmd, "insecure", "PLATFORMCTL_INSECURE", os.Getenv("PLATFORMCTL_INSECURE"), f.Insecure, fc.Insecure)
	if err != nil {
		return err
	}
	f.Insecure = insecure

	// Normalize the server URL: strip a trailing slash so path joining is clean.
	f.Server = strings.TrimRight(f.Server, "/")
	if f.Server == "" {
		return fmt.Errorf("server URL must not be empty")
	}

	switch f.Output {
	case "table", "json", "yaml":
	default:
		return fmt.Errorf("invalid output format %q (want table, json or yaml)", f.Output)
	}

	return nil
}

// resolveStr picks the first non-empty value in precedence order: an explicitly
// set flag, an environment variable, the config file, then the default.
func resolveStr(cmd *cobra.Command, name, flagVal, envVal, fileVal, def string) string {
	if cmd.Flags().Changed(name) {
		return flagVal
	}
	if envVal != "" {
		return envVal
	}
	if fileVal != "" {
		return fileVal
	}
	return def
}

// resolveBool applies the same precedence to a boolean. Environment values
// accept 1/t/true/y/yes/on (true) and 0/f/false/n/no/off (false), case-
// insensitively; an unrecognized environment value is a hard error rather than
// being silently dropped (this toggle is security-sensitive).
func resolveBool(cmd *cobra.Command, name, envName, envVal string, flagVal, fileVal bool) (bool, error) {
	if cmd.Flags().Changed(name) {
		return flagVal, nil
	}
	if envVal != "" {
		b, ok := parseBoolLenient(envVal)
		if !ok {
			return false, fmt.Errorf("invalid boolean value %q for %s (use true or false)", envVal, envName)
		}
		return b, nil
	}
	return fileVal, nil
}

// parseBoolLenient parses common truthy/falsy tokens. The second return value
// reports whether the input was recognized.
func parseBoolLenient(s string) (value bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "t", "true", "y", "yes", "on":
		return true, true
	case "0", "f", "false", "n", "no", "off":
		return false, true
	}
	return false, false
}

// loadFileConfig reads and parses the config file. A missing default config
// file is not an error; a missing explicitly requested (--config) file is.
func loadFileConfig(explicitPath string) (fileConfig, error) {
	var fc fileConfig

	path := explicitPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// No home directory: just fall back to defaults/env.
			return fc, nil
		}
		path = filepath.Join(home, ".platformctl.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && explicitPath == "" {
			return fc, nil // default file simply doesn't exist
		}
		return fc, fmt.Errorf("reading config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fc, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return fc, nil
}
