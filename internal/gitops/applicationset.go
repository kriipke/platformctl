package gitops

import "time"

// ApplicationSetStatus tracks ApplicationSet health for a context/environment.
type ApplicationSetStatus struct {
	ContextName string     `json:"contextName"`
	Name        string     `json:"name"`
	Namespace   string     `json:"namespace"`
	Cluster     string     `json:"cluster"`
	Status      string     `json:"status"`
	Health      string     `json:"health"`
	LastSync    *time.Time `json:"lastSync,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
}
