package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Secret represents an encrypted tenant secret.
type Secret struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Key          string    `json:"key"`
	Value        string    `json:"value"`
	EncryptedValue string   `json:"encrypted_value"`
	Description  string    `json:"description"`
	Tags         []string  `json:"tags"`
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SecretMetadata is the lightweight representation for listing secrets.
type SecretMetadata struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Description string   `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Version     int       `json:"version"`
	VersionCount int     `json:"version_count"`
}

// SecretStore manages encrypted secrets per tenant.
type SecretStore struct {
	mu      sync.RWMutex
	secrets map[string]*Secret // keyed by auto-generated UUID
	byKey   map[string]string  // keyed by "tenantID:key"
}

// NewSecretStore creates a new SecretStore.
func NewSecretStore() *SecretStore {
	return &SecretStore{
		secrets: make(map[string]*Secret),
		byKey:   make(map[string]string),
	}
}

// Create adds a new secret. The id parameter is the tenant ID.
func (s *SecretStore) Create(tenantID, key, value, description string, tags []string) (*Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key == "" {
		return nil, fmt.Errorf("secret key is required")
	}

	lookupKey := tenantID + ":" + key
	if _, exists := s.byKey[lookupKey]; exists {
		return nil, fmt.Errorf("secret key %q already exists", key)
	}

	now := timeNow()
	ver := 1
	// Find version count for this tenant+key
	for _, sec := range s.secrets {
		if sec.Key == key && sec.TenantID == tenantID {
			if sec.Version >= ver {
				ver = sec.Version + 1
			}
		}
	}

	id := uuid.New().String()
	encVal, err := encryptValue(tenantID, value)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}
	sec := &Secret{
		ID:             id,
		TenantID:       tenantID,
		Key:            key,
		Value:          value,
		EncryptedValue: encVal,
		Description:    description,
		Tags:           tags,
		Version:        ver,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.secrets[sec.ID] = sec
	s.byKey[lookupKey] = sec.ID

	return sec, nil
}

// GetByID retrieves a secret by ID (returns value; no tenant check — for admin use only).
func (s *SecretStore) GetByID(id string) (*Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sec, ok := s.secrets[id]
	if !ok {
		return nil, fmt.Errorf("secret %s not found", id)
	}
	cpy := *sec
	return &cpy, nil
}

// GetByIDAndTenant retrieves a secret by ID and verifies the TenantID matches.
func (s *SecretStore) GetByIDAndTenant(id, tenantID string) (*Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sec, ok := s.secrets[id]
	if !ok {
		return nil, fmt.Errorf("secret %s not found", id)
	}
	if sec.TenantID != tenantID {
		return nil, fmt.Errorf("permission denied: secret %s does not belong to tenant %s", id, tenantID)
	}
	cpy := *sec
	return &cpy, nil
}

// List returns metadata-only list of secrets for a tenant.
func (s *SecretStore) List(tenantID string, page, pageSize int) ([]*SecretMetadata, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*SecretMetadata, 0)
	for _, sec := range s.secrets {
		if sec.TenantID != tenantID {
			continue
		}
		// Count versions
		verCount := 1
		for _, other := range s.secrets {
			if other.Key == sec.Key && other.TenantID == tenantID && other.Version > verCount {
				verCount = other.Version
			}
		}

		meta := &SecretMetadata{
			ID:           sec.ID,
			Key:          sec.Key,
			Description:  sec.Description,
			Tags:         sec.Tags,
			CreatedAt:    sec.CreatedAt,
			UpdatedAt:    sec.UpdatedAt,
			Version:      sec.Version,
			VersionCount: verCount,
		}
		items = append(items, meta)
	}

	total := len(items)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	slices.SortFunc(items, func(a, b *SecretMetadata) int {
		return a.KeyCompare(b.Key)
	})

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := items[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

// Update modifies secret metadata (description, tags) without changing the value.
func (s *SecretStore) Update(id string, description string, tags []string) (*Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sec, ok := s.secrets[id]
	if !ok {
		return nil, fmt.Errorf("secret %s not found", id)
	}

	sec.Description = description
	sec.Tags = tags
	sec.UpdatedAt = timeNow()

	return sec, nil
}

// Rotate creates a new version of the secret with a new value.
func (s *SecretStore) Rotate(id, newValue string) (*Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sec, ok := s.secrets[id]
	if !ok {
		return nil, fmt.Errorf("secret %s not found", id)
	}

	oldKey := sec.Key
	oldID := sec.ID

	encVal, err := encryptValue(sec.TenantID, newValue)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	newSec := &Secret{
		ID:             oldID,
		Key:            oldKey,
		Value:          newValue,
		EncryptedValue: encVal,
		Description:    sec.Description,
		Tags:           sec.Tags,
		Version:        sec.Version + 1,
		CreatedAt:      sec.CreatedAt,
		UpdatedAt:      timeNow(),
	}

	s.secrets[newSec.ID] = newSec

	return newSec, nil
}

// Delete removes a secret.
func (s *SecretStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sec, ok := s.secrets[id]
	if !ok {
		return fmt.Errorf("secret %s not found", id)
	}

	deleteKey := sec.TenantID + ":" + sec.Key
	delete(s.secrets, id)
	delete(s.byKey, deleteKey)

	return nil
}

func (m *SecretMetadata) KeyCompare(otherKey string) int {
	if m.Key < otherKey {
		return -1
	}
	if m.Key > otherKey {
		return 1
	}
	return 0
}

// encryptValue encrypts plaintext using AES-256-GCM.
// The key is hashed to exactly 32 bytes (AES-256 key size) using SHA-256.
func encryptValue(key, plaintext string) (string, error) {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	aesKey := hasher.Sum(nil) // 32 bytes = AES-256

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return fmt.Sprintf("AES256:%x", ciphertext), nil
}
