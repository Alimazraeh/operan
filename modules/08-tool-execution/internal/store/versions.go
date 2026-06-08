package store

import (
	"sort"
	"sync"

	"github.com/google/uuid"
)

// VersionStore tracks immutable tool versions.
type VersionStore struct {
	mu     sync.RWMutex
	byTool map[string][]*ToolVersion // toolID -> versions
}

// NewVersionStore creates an empty VersionStore.
func NewVersionStore() *VersionStore {
	return &VersionStore{byTool: make(map[string][]*ToolVersion)}
}

// Create records a new tool version.
func (s *VersionStore) Create(v *ToolVersion) (*ToolVersion, error) {
	if v.ToolID == "" {
		return nil, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.Status == "" {
		v.Status = "active"
	}
	v.CreatedAt = timeNow()
	s.byTool[v.ToolID] = append(s.byTool[v.ToolID], v)
	cp := *v
	return &cp, nil
}

// ListByTool returns versions for a tool, newest first, paginated.
func (s *VersionStore) ListByTool(toolID string, page, pageSize int) ([]ToolVersion, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := s.byTool[toolID]
	all := make([]*ToolVersion, len(versions))
	copy(all, versions)
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]ToolVersion, 0, end-start)
	for _, v := range all[start:end] {
		out = append(out, *v)
	}
	return out, total, end < total
}
