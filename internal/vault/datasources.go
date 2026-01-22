package vault

// Datasource describes a Vault datasource for environment configuration.
type Datasource struct {
	Vault string   `json:"vault"`
	Keys  []string `json:"keys"`
}
