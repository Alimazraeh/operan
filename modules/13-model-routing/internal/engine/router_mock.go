package engine

import (
	"github.com/operan/model-routing/internal/store"
)

// mockRuleStore implements RuleStore for tests.
type mockRuleStore struct {
	rules []store.RuleWithModels
}

func (m *mockRuleStore) ListActiveRulesByTask(_, taskType string) ([]store.RuleWithModels, error) {
	var result []store.RuleWithModels
	for _, r := range m.rules {
		if r.TaskType == taskType {
			result = append(result, r)
		}
	}
	return result, nil
}

// mockPerfStore implements PerfStore for tests.
type mockPerfStore struct {
	records map[string]*store.RoutingPerformance
}

func newMockPerfStore() *mockPerfStore {
	return &mockPerfStore{records: make(map[string]*store.RoutingPerformance)}
}

func (m *mockPerfStore) key(tenantID, modelID, taskType string) string {
	return tenantID + "|" + modelID + "|" + taskType
}

func (m *mockPerfStore) GetByModelAndTask(tenantID, modelID, taskType string) (*store.RoutingPerformance, error) {
	r, ok := m.records[m.key(tenantID, modelID, taskType)]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockPerfStore) record(mKey string, metric *store.RoutingPerformance) {
	m.records[mKey] = metric
}