package boundary

import "strings"

// Scope represents a Boundary scope.
type Scope struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	ParentScopeID string `json:"parent_scope_id"`
}

// HostCatalog represents a static host catalog.
type HostCatalog struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	ScopeID string `json:"scope_id"`
}

// Host represents a static host entry.
type Host struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	Type        string   `json:"type,omitempty"`
	HostSetIDs  []string `json:"host_set_ids,omitempty"`
	IPAddresses []string `json:"ip_addresses,omitempty"`
	DNSNames    []string `json:"dns_names,omitempty"`
}

// HostSet represents a static host set.
type HostSet struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	HostCatalogID string   `json:"host_catalog_id,omitempty"`
	HostIDs       []string `json:"host_ids,omitempty"`
}

// Target represents a Boundary target.
type Target struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	ScopeID       string   `json:"scope_id"`
	DefaultPort   int      `json:"default_port"`
	Address       string   `json:"address,omitempty"`
	HostSourceIDs []string `json:"host_source_ids,omitempty"`
}

// TargetInspection contains the target plus its backing host source details.
type TargetInspection struct {
	Target      Target                    `json:"target"`
	HostSources []TargetHostSourceDetails `json:"host_sources,omitempty"`
}

// TargetHostSourceDetails describes one host set attached to a target.
type TargetHostSourceDetails struct {
	HostSet     HostSet     `json:"host_set"`
	HostCatalog HostCatalog `json:"host_catalog"`
	Hosts       []Host      `json:"hosts,omitempty"`
}

// Role represents a Boundary role.
type Role struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	ScopeID string `json:"scope_id"`
}

// CreateSSHBundleRequest contains the input for the guided SSH setup flow.
type CreateSSHBundleRequest struct {
	ScopeID     string
	CatalogName string
	HostName    string
	HostAddress string
	HostSetName string
	TargetName  string
	Port        int
	CreateRole  bool
	RoleName    string
	PrincipalID string
}

// CreatedSSHBundle contains the resources created during setup.
type CreatedSSHBundle struct {
	Catalog HostCatalog
	Host    Host
	HostSet HostSet
	Target  Target
	Role    *Role
}

// FilterProjectScopes returns only project scopes.
func FilterProjectScopes(scopes []Scope) []Scope {
	projects := make([]Scope, 0, len(scopes))
	for _, scope := range scopes {
		if strings.EqualFold(scope.Type, "project") {
			projects = append(projects, scope)
		}
	}

	return projects
}

// FindTargetByName locates a target using an exact or case-insensitive name match.
func FindTargetByName(targets []Target, name string) (Target, bool) {
	matches := FindTargetsByName(targets, name)
	if len(matches) == 0 {
		return Target{}, false
	}

	return matches[0], true
}

// FindTargetsByName returns all exact matches, or case-insensitive matches when
// no exact match exists.
func FindTargetsByName(targets []Target, name string) []Target {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}

	exactMatches := make([]Target, 0, len(targets))
	for _, target := range targets {
		if target.Name == trimmed {
			exactMatches = append(exactMatches, target)
		}
	}
	if len(exactMatches) > 0 {
		return exactMatches
	}

	foldedMatches := make([]Target, 0, len(targets))
	for _, target := range targets {
		if strings.EqualFold(target.Name, trimmed) {
			foldedMatches = append(foldedMatches, target)
		}
	}

	return foldedMatches
}
