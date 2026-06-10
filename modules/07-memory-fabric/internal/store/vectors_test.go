package store

import (
	"testing"
	"time"
)

func newVector(tenant, doc, content string, et EmbeddingType) *MemoryVector {
	return &MemoryVector{
		TenantID:        tenant,
		DocumentID:      doc,
		SemanticContent: content,
		EmbeddingType:   et,
	}
}

func TestVectorCreateAndGet(t *testing.T) {
	s := NewVectorStore()
	v, err := s.Create(newVector("t1", "d1", "the quarterly report", EmbeddingDepartment))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.ID == "" || v.VectorHash == "" || v.CreatedAt.IsZero() {
		t.Error("Create should assign ID, hash, and timestamp")
	}

	got, err := s.GetByIDAndTenant(v.ID, "t1")
	if err != nil {
		t.Fatalf("GetByIDAndTenant: %v", err)
	}
	if got.SemanticContent != "the quarterly report" {
		t.Errorf("content = %q", got.SemanticContent)
	}
}

func TestVectorTenantIsolation(t *testing.T) {
	s := NewVectorStore()
	v, _ := s.Create(newVector("t1", "d1", "secret", EmbeddingPlatform))

	if _, err := s.GetByIDAndTenant(v.ID, "t2"); err != ErrNotFound {
		t.Errorf("cross-tenant Get should return ErrNotFound, got %v", err)
	}
	if err := s.Delete(v.ID, "t2"); err != ErrNotFound {
		t.Errorf("cross-tenant Delete should return ErrNotFound, got %v", err)
	}
	if items, total, _ := s.List("t2", 1, 10, nil, nil, nil); total != 0 || len(items) != 0 {
		t.Errorf("cross-tenant List should be empty, got %d items", total)
	}
}

func TestVectorCreateValidation(t *testing.T) {
	s := NewVectorStore()
	cases := []*MemoryVector{
		newVector("", "d1", "x", EmbeddingPlatform),            // no tenant
		newVector("t1", "", "x", EmbeddingPlatform),            // no document
		newVector("t1", "d1", "", EmbeddingPlatform),           // no content
		newVector("t1", "d1", "x", EmbeddingType("bogus")),     // bad type
		{TenantID: "t1", DocumentID: "d1", SemanticContent: "x", EmbeddingType: EmbeddingPlatform, SegmentType: SegmentType("bogus")},
	}
	for i, c := range cases {
		if _, err := s.Create(c); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

func TestVectorListFiltersAndPagination(t *testing.T) {
	s := NewVectorStore()
	for i := 0; i < 5; i++ {
		s.Create(newVector("t1", "d1", "alpha", EmbeddingDepartment))
	}
	v := newVector("t1", "d2", "beta", EmbeddingPlatform)
	v.SegmentType = SegmentFact
	s.Create(v)

	et := "department"
	if _, total, _ := s.List("t1", 1, 10, &et, nil, nil); total != 5 {
		t.Errorf("embedding_type filter: total = %d, want 5", total)
	}
	st := "fact"
	if _, total, _ := s.List("t1", 1, 10, nil, &st, nil); total != 1 {
		t.Errorf("segment_type filter: total = %d, want 1", total)
	}
	doc := "d2"
	if _, total, _ := s.List("t1", 1, 10, nil, nil, &doc); total != 1 {
		t.Errorf("document_id filter: total = %d, want 1", total)
	}

	items, total, hasMore := s.List("t1", 1, 4, nil, nil, nil)
	if total != 6 || len(items) != 4 || !hasMore {
		t.Errorf("page 1: total=%d len=%d hasMore=%v, want 6/4/true", total, len(items), hasMore)
	}
	items, _, hasMore = s.List("t1", 2, 4, nil, nil, nil)
	if len(items) != 2 || hasMore {
		t.Errorf("page 2: len=%d hasMore=%v, want 2/false", len(items), hasMore)
	}
}

func TestVectorUpdate(t *testing.T) {
	s := NewVectorStore()
	v, _ := s.Create(newVector("t1", "d1", "old content", EmbeddingDepartment))
	oldHash := v.VectorHash

	updated, err := s.Update(v.ID, "t1", func(m *MemoryVector) error {
		m.SemanticContent = "new content"
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.SemanticContent != "new content" {
		t.Errorf("content = %q", updated.SemanticContent)
	}
	if updated.VectorHash == oldHash {
		t.Error("hash should change when content changes")
	}

	if _, err := s.Update(v.ID, "t2", func(m *MemoryVector) error { return nil }); err != ErrNotFound {
		t.Errorf("cross-tenant Update should return ErrNotFound, got %v", err)
	}
}

func TestSearchCosineSimilarity(t *testing.T) {
	s := NewVectorStore()
	a := newVector("t1", "d1", "a", EmbeddingDepartment)
	a.EmbeddingVector = []float64{1, 0, 0}
	s.Create(a)
	b := newVector("t1", "d2", "b", EmbeddingDepartment)
	b.EmbeddingVector = []float64{0, 1, 0}
	s.Create(b)

	results := s.Search("t1", "ignored", []float64{1, 0, 0}, "department", 10, 0.5, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result above threshold, got %d", len(results))
	}
	if results[0].Vector.DocumentID != "d1" || results[0].Score < 0.99 {
		t.Errorf("top result = %s score %f, want d1 ~1.0", results[0].Vector.DocumentID, results[0].Score)
	}
}

func TestSearchTokenOverlapFallback(t *testing.T) {
	s := NewVectorStore()
	s.Create(newVector("t1", "d1", "The customer demo is on Friday", EmbeddingDepartment))
	s.Create(newVector("t1", "d2", "Unrelated platform telemetry", EmbeddingDepartment))

	results := s.Search("t1", "customer demo", nil, "department", 10, 0.5, nil)
	if len(results) != 1 || results[0].Vector.DocumentID != "d1" {
		t.Fatalf("expected only d1, got %d results", len(results))
	}
}

func TestSearchTokenPrefixMatching(t *testing.T) {
	s := NewVectorStore()
	s.Create(newVector("t1", "d1", "Customer prefers Arabic-first UI in demos", EmbeddingAgentPersonal))

	// "demo" should match "demos", "arabic" should match "arabic-first".
	results := s.Search("t1", "Arabic demo", nil, "agent_personal", 10, 0.5, nil)
	if len(results) != 1 {
		t.Fatalf("prefix matching: expected 1 result, got %d", len(results))
	}
	if results[0].Score < 0.99 {
		t.Errorf("score = %f, want 1.0 (both tokens matched)", results[0].Score)
	}

	// Short tokens (<4 chars) must still require exact match: "in" matches,
	// "de" must not match "demos".
	if r := s.Search("t1", "de", nil, "agent_personal", 10, 0.5, nil); len(r) != 0 {
		t.Errorf("short prefix should not match, got %d results", len(r))
	}
}

func TestSearchScopesAndIDFilter(t *testing.T) {
	s := NewVectorStore()
	v1, _ := s.Create(newVector("t1", "d1", "same words here", EmbeddingDepartment))
	s.Create(newVector("t1", "d2", "same words here", EmbeddingDepartment))
	s.Create(newVector("t1", "d3", "same words here", EmbeddingPlatform))
	s.Create(newVector("t2", "d4", "same words here", EmbeddingDepartment))

	all := s.Search("t1", "same words", nil, "department", 10, 0, nil)
	if len(all) != 2 {
		t.Errorf("scope: got %d results, want 2 (embedding type + tenant filter)", len(all))
	}
	only := s.Search("t1", "same words", nil, "department", 10, 0, []string{v1.ID})
	if len(only) != 1 || only[0].Vector.ID != v1.ID {
		t.Errorf("vector_ids filter: got %d results", len(only))
	}
	if topped := s.Search("t1", "same words", nil, "department", 1, 0, nil); len(topped) != 1 {
		t.Errorf("top_n: got %d results, want 1", len(topped))
	}
}

func TestCollectExpired(t *testing.T) {
	s := NewVectorStore()
	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)

	expired := newVector("t1", "d1", "old", EmbeddingAgentEphemeral)
	expired.TTL = &past
	s.Create(expired)
	fresh := newVector("t1", "d2", "new", EmbeddingAgentEphemeral)
	fresh.TTL = &future
	s.Create(fresh)
	noTTL := newVector("t1", "d3", "keep", EmbeddingDepartment)
	s.Create(noTTL)

	// Dry run removes nothing.
	ids := s.CollectExpired("t1", nil, 0, 100, true)
	if len(ids) != 1 {
		t.Fatalf("dry run: collected %d, want 1", len(ids))
	}
	if _, total, _ := s.List("t1", 1, 10, nil, nil, nil); total != 3 {
		t.Errorf("dry run should not delete; total = %d, want 3", total)
	}

	// Real run removes the expired vector only.
	ids = s.CollectExpired("t1", nil, 0, 100, false)
	if len(ids) != 1 {
		t.Fatalf("collected %d, want 1", len(ids))
	}
	if _, total, _ := s.List("t1", 1, 10, nil, nil, nil); total != 2 {
		t.Errorf("after GC total = %d, want 2", total)
	}
}

func TestCollectExpiredByTypeAndAge(t *testing.T) {
	s := NewVectorStore()
	old := newVector("t1", "d1", "ancient", EmbeddingDepartment)
	s.Create(old)
	// Backdate creation by mutating through Update (test-only shortcut).
	s.Update(old.ID, "t1", func(m *MemoryVector) error {
		m.CreatedAt = time.Now().UTC().AddDate(0, 0, -30)
		return nil
	})
	s.Create(newVector("t1", "d2", "recent", EmbeddingDepartment))

	et := EmbeddingPlatform
	if ids := s.CollectExpired("t1", &et, 7, 100, false); len(ids) != 0 {
		t.Errorf("wrong-type filter collected %d, want 0", len(ids))
	}
	et = EmbeddingDepartment
	ids := s.CollectExpired("t1", &et, 7, 100, false)
	if len(ids) != 1 {
		t.Errorf("max_age_days collected %d, want 1", len(ids))
	}
}

func TestAgentMemoryIDs(t *testing.T) {
	s := NewVectorStore()
	p1 := newVector("t1", "d1", "memory one", EmbeddingAgentPersonal)
	p1.Metadata = map[string]interface{}{"agent_id": "agent-1"}
	s.Create(p1)
	p2 := newVector("t1", "d2", "memory two", EmbeddingAgentPersonal)
	p2.Metadata = map[string]interface{}{"agent_id": "agent-1"}
	s.Create(p2)
	eph := newVector("t1", "d3", "scratch", EmbeddingAgentEphemeral)
	eph.Metadata = map[string]interface{}{"agent_id": "agent-1"}
	s.Create(eph)
	other := newVector("t1", "d4", "not mine", EmbeddingAgentPersonal)
	other.Metadata = map[string]interface{}{"agent_id": "agent-2"}
	s.Create(other)

	ids, latest, found := s.AgentMemoryIDs("t1", "agent-1")
	if !found {
		t.Fatal("agent-1 should be found")
	}
	if len(ids) != 2 {
		t.Errorf("personal memories = %d, want 2 (ephemeral excluded)", len(ids))
	}
	if latest == nil {
		t.Error("latest should be set")
	}

	if _, _, found := s.AgentMemoryIDs("t1", "agent-x"); found {
		t.Error("unknown agent should not be found")
	}
	if _, _, found := s.AgentMemoryIDs("t2", "agent-1"); found {
		t.Error("agent should not be found in other tenant")
	}
}
