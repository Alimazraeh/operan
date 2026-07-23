package store

import "encoding/json"

// Snapshot Export/Import for the persist package. Each store dumps its full
// maps (entities + indexes) as one JSON document; Import replaces state
// wholesale. VersionStore's byTemplate/byTenant hold pointers into versions,
// so its snapshot stores IDs and rebuilds the indexes on Import.

// ─── TemplateStore ───────────────────────────────────────────────────────────

type templateSnapshot struct {
	Templates map[string]*Template `json:"templates"`
	ByTenant  map[string][]string  `json:"by_tenant"`
}

func (s *TemplateStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(templateSnapshot{Templates: s.templates, ByTenant: s.byTenant})
}

func (s *TemplateStore) Import(data []byte) error {
	var snap templateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap.Templates != nil {
		s.templates = snap.Templates
	}
	if snap.ByTenant != nil {
		s.byTenant = snap.ByTenant
	}
	return nil
}

// ─── CustomTemplateStore ─────────────────────────────────────────────────────

type customTemplateSnapshot struct {
	Templates map[string]*CustomTemplate `json:"templates"`
	ByTenant  map[string][]string        `json:"by_tenant"`
}

func (s *CustomTemplateStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(customTemplateSnapshot{Templates: s.templates, ByTenant: s.byTenant})
}

func (s *CustomTemplateStore) Import(data []byte) error {
	var snap customTemplateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap.Templates != nil {
		s.templates = snap.Templates
	}
	if snap.ByTenant != nil {
		s.byTenant = snap.ByTenant
	}
	return nil
}

// ─── DeploymentStore ─────────────────────────────────────────────────────────

type deploymentSnapshot struct {
	Deployments map[string]*TemplateDeployment `json:"deployments"`
	ByTemplate  map[string][]string            `json:"by_template"`
	ByTenant    map[string][]string            `json:"by_tenant"`
}

func (s *DeploymentStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(deploymentSnapshot{Deployments: s.deployments, ByTemplate: s.byTemplate, ByTenant: s.byTenant})
}

func (s *DeploymentStore) Import(data []byte) error {
	var snap deploymentSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap.Deployments != nil {
		s.deployments = snap.Deployments
	}
	if snap.ByTemplate != nil {
		s.byTemplate = snap.ByTemplate
	}
	if snap.ByTenant != nil {
		s.byTenant = snap.ByTenant
	}
	return nil
}

// ─── VersionStore ────────────────────────────────────────────────────────────

type versionSnapshot struct {
	Versions map[string]*TemplateVersion `json:"versions"`
}

func (s *VersionStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(versionSnapshot{Versions: s.versions})
}

func (s *VersionStore) Import(data []byte) error {
	var snap versionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap.Versions == nil {
		return nil
	}
	s.versions = snap.Versions
	// Rebuild pointer indexes from the restored versions map.
	s.byTemplate = make(map[string][]*TemplateVersion)
	s.byTenant = make(map[string][]*TemplateVersion)
	for _, v := range s.versions {
		s.byTemplate[v.TemplateID] = append(s.byTemplate[v.TemplateID], v)
		s.byTenant[v.TenantID] = append(s.byTenant[v.TenantID], v)
	}
	return nil
}
