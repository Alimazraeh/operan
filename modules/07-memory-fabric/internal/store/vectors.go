package store

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// VectorStore provides tenant-isolated storage for memory vectors.
type VectorStore struct {
	mu       sync.RWMutex
	vectors  map[string]*MemoryVector // id -> vector
	byTenant map[string][]string      // tenantID -> []vectorID
}

// NewVectorStore creates an empty VectorStore.
func NewVectorStore() *VectorStore {
	return &VectorStore{
		vectors:  make(map[string]*MemoryVector),
		byTenant: make(map[string][]string),
	}
}

// Create stores a new memory vector, generating ID, hash, and timestamp.
func (s *VectorStore) Create(v *MemoryVector) (*MemoryVector, error) {
	if v.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if v.DocumentID == "" || v.SemanticContent == "" || !ValidEmbeddingType(string(v.EmbeddingType)) {
		return nil, ErrValidation
	}
	if v.SegmentType != "" && !ValidSegmentType(string(v.SegmentType)) {
		return nil, ErrValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.Metadata == nil {
		v.Metadata = map[string]interface{}{}
	}
	v.CreatedAt = timeNow()
	v.VectorHash = hashVector(v.SemanticContent, v.EmbeddingVector)

	s.vectors[v.ID] = v
	s.byTenant[v.TenantID] = append(s.byTenant[v.TenantID], v.ID)
	cp := *v
	return &cp, nil
}

// GetByIDAndTenant returns a vector scoped to a tenant.
func (s *VectorStore) GetByIDAndTenant(id, tenantID string) (*MemoryVector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vectors[id]
	if !ok || v.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *v
	return &cp, nil
}

// Update applies a partial update to a tenant's vector via the upd closure.
func (s *VectorStore) Update(id, tenantID string, upd func(*MemoryVector) error) (*MemoryVector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.vectors[id]
	if !ok || v.TenantID != tenantID {
		return nil, ErrNotFound
	}
	if err := upd(v); err != nil {
		return nil, err
	}
	v.VectorHash = hashVector(v.SemanticContent, v.EmbeddingVector)
	cp := *v
	return &cp, nil
}

// Delete removes a tenant's vector.
func (s *VectorStore) Delete(id, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.vectors[id]
	if !ok || v.TenantID != tenantID {
		return ErrNotFound
	}
	delete(s.vectors, id)
	s.removeFromTenantLocked(tenantID, id)
	return nil
}

// List returns a tenant's vectors with optional filters, paginated and
// ordered by creation time (newest first).
func (s *VectorStore) List(tenantID string, page, pageSize int, embeddingType, segmentType, documentID *string) ([]MemoryVector, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []*MemoryVector
	for _, id := range s.byTenant[tenantID] {
		v := s.vectors[id]
		if v == nil {
			continue
		}
		if embeddingType != nil && string(v.EmbeddingType) != *embeddingType {
			continue
		}
		if segmentType != nil && string(v.SegmentType) != *segmentType {
			continue
		}
		if documentID != nil && v.DocumentID != *documentID {
			continue
		}
		matched = append(matched, v)
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].CreatedAt.After(matched[j].CreatedAt) })

	total := len(matched)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	out := make([]MemoryVector, 0, end-start)
	for _, v := range matched[start:end] {
		out = append(out, *v)
	}
	return out, total, end < total
}

// ScoredVector pairs a vector with its search relevance score.
type ScoredVector struct {
	Vector MemoryVector
	Score  float64
}

// Search ranks a tenant's vectors against the query. When both the query
// vector and stored vectors have embeddings, cosine similarity is used;
// otherwise a deterministic token-overlap score over semantic content.
// Real embedding generation belongs to Module 12 (model abstraction).
func (s *VectorStore) Search(tenantID, query string, queryVector []float64, embeddingType string, topN int, threshold float64, vectorIDs []string) []ScoredVector {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idFilter := map[string]bool{}
	for _, id := range vectorIDs {
		idFilter[id] = true
	}

	var results []ScoredVector
	for _, id := range s.byTenant[tenantID] {
		v := s.vectors[id]
		if v == nil || string(v.EmbeddingType) != embeddingType {
			continue
		}
		if len(idFilter) > 0 && !idFilter[v.ID] {
			continue
		}

		var score float64
		if len(queryVector) > 0 && len(v.EmbeddingVector) == len(queryVector) {
			score = cosineSimilarity(queryVector, v.EmbeddingVector)
		} else {
			score = tokenOverlap(query, v.SemanticContent)
		}
		if score < threshold {
			continue
		}
		results = append(results, ScoredVector{Vector: *v, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Vector.ID < results[j].Vector.ID
	})
	if topN > 0 && len(results) > topN {
		results = results[:topN]
	}
	return results
}

// CollectExpired removes up to limit expired vectors for a tenant
// (TTL in the past, optionally restricted to one embedding type and a
// minimum age in days). When dryRun is set, nothing is removed.
// Returns the IDs that were (or would be) collected.
func (s *VectorStore) CollectExpired(tenantID string, embeddingType *EmbeddingType, maxAgeDays int, limit int, dryRun bool) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeNow()
	var collected []string
	for _, id := range s.byTenant[tenantID] {
		if limit > 0 && len(collected) >= limit {
			break
		}
		v := s.vectors[id]
		if v == nil {
			continue
		}
		if embeddingType != nil && v.EmbeddingType != *embeddingType {
			continue
		}
		expired := v.TTL != nil && v.TTL.Before(now)
		tooOld := maxAgeDays > 0 && v.CreatedAt.Before(now.AddDate(0, 0, -maxAgeDays))
		if !expired && !tooOld {
			continue
		}
		collected = append(collected, id)
	}

	if !dryRun {
		for _, id := range collected {
			delete(s.vectors, id)
		}
		for _, id := range collected {
			s.removeFromTenantLocked(tenantID, id)
		}
	}
	return collected
}

// AgentMemoryIDs returns the personal-memory vector IDs for an agent
// (vectors of type agent_personal whose metadata agent_id matches),
// plus the most recent update time. Found reports whether any vector
// references the agent at all.
func (s *VectorStore) AgentMemoryIDs(tenantID, agentID string) (ids []string, lastUpdated *MemoryVector, found bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, id := range s.byTenant[tenantID] {
		v := s.vectors[id]
		if v == nil {
			continue
		}
		va, _ := v.Metadata["agent_id"].(string)
		if va != agentID {
			continue
		}
		found = true
		if v.EmbeddingType != EmbeddingAgentPersonal {
			continue
		}
		ids = append(ids, v.ID)
		if lastUpdated == nil || v.CreatedAt.After(lastUpdated.CreatedAt) {
			cp := *v
			lastUpdated = &cp
		}
	}
	sort.Strings(ids)
	return ids, lastUpdated, found
}

func (s *VectorStore) removeFromTenantLocked(tenantID, id string) {
	list := s.byTenant[tenantID]
	for i, vid := range list {
		if vid == id {
			s.byTenant[tenantID] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// ─── Scoring helpers ─────────────────────────────────────────────────────────

func cosineSimilarity(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	sim := dot / (math.Sqrt(na) * math.Sqrt(nb))
	// Clamp to [0,1] per the SearchResult.score contract.
	if sim < 0 {
		return 0
	}
	if sim > 1 {
		return 1
	}
	return sim
}

// tokenOverlap scores by the fraction of distinct query tokens present in
// the content (case-insensitive). A token matches exactly or when one token
// is a ≥4-character prefix of the other ("demo" matches "demos",
// "arabic" matches "arabic-first"), which absorbs simple plural/compound
// variation without a stemmer.
func tokenOverlap(query, content string) float64 {
	qTokens := strings.Fields(strings.ToLower(query))
	if len(qTokens) == 0 {
		return 0
	}
	var cTokens []string
	for _, t := range strings.Fields(strings.ToLower(content)) {
		if t = strings.Trim(t, ".,;:!?\"'()"); t != "" {
			cTokens = append(cTokens, t)
		}
	}
	distinct := map[string]bool{}
	hits := 0
	for _, q := range qTokens {
		q = strings.Trim(q, ".,;:!?\"'()")
		if q == "" || distinct[q] {
			continue
		}
		distinct[q] = true
		for _, c := range cTokens {
			if tokensMatch(q, c) {
				hits++
				break
			}
		}
	}
	if len(distinct) == 0 {
		return 0
	}
	return float64(hits) / float64(len(distinct))
}

func tokensMatch(a, b string) bool {
	if a == b {
		return true
	}
	short, long := a, b
	if len(short) > len(long) {
		short, long = long, short
	}
	return len(short) >= 4 && strings.HasPrefix(long, short)
}

func hashVector(content string, embedding []float64) string {
	h := sha256.New()
	h.Write([]byte(content))
	var buf [8]byte
	for _, f := range embedding {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(f))
		h.Write(buf[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}
