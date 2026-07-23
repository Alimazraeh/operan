package store

// Mutate applies fn to the stored deployment under the write lock and returns
// a copy of the result. Used by the deploy orchestrator to record stage
// history, provisioned entities and the department link atomically.
func (s *DeploymentStore) Mutate(id string, fn func(*TemplateDeployment)) (*TemplateDeployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deployments[id]
	if !ok {
		return nil, ErrNotFound
	}
	fn(d)
	d.UpdatedAt = timeNow()
	cp := *d
	return &cp, nil
}
