package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultJWKSTTL is the default time-to-live for JWKS cache entries.
const DefaultJWKSTTL = time.Hour

// jwksResponse represents the well-known JWKS endpoint response.
type jwksResponse struct {
	Keys []json.RawMessage `json:"keys"`
}

// jwkEntry represents a single key in a JWKS response.
type jwkEntry struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwkPublicKey represents a cached public key entry.
type jwkPublicKey struct {
	Key       *rsa.PublicKey
	KeyID     string
	ExpiresAt time.Time
}

// JWKSCache is a thread-safe cache for JWKS public keys with automatic refresh.
type JWKSCache struct {
	mu          sync.RWMutex
	keys        map[string]*jwkPublicKey
	lastRefresh time.Time
	jwksURL     string
	httpClient  *http.Client
	ttl         time.Duration
	refreshCtx    context.Context
	refreshCancel context.CancelFunc
	refreshing  bool
}

// NewJWKSCache creates a new JWKS cache with the given JWKS URL and HTTP client.
func NewJWKSCache(jwksURL string, httpClient *http.Client) *JWKSCache {
	return &JWKSCache{
		keys:       make(map[string]*jwkPublicKey),
		jwksURL:    jwksURL,
		httpClient: httpClient,
		ttl:        DefaultJWKSTTL,
	}
}

// Get retrieves a cached public key by key ID.
// Returns the key and true if found, or nil and false if not present.
func (c *JWKSCache) Get(keyID string) (*jwkPublicKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key, ok := c.keys[keyID]
	return key, ok
}

// Set stores a public key in the cache with an expiration time.
func (c *JWKSCache) Set(keyID string, key *jwkPublicKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys[keyID] = key
}

// IsExpired returns true if the cache has not been refreshed within the TTL.
func (c *JWKSCache) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lastRefresh.IsZero() {
		return true
	}
	return time.Since(c.lastRefresh) > c.ttl
}

// LastRefresh returns the time of the last successful JWKS refresh.
func (c *JWKSCache) LastRefresh() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastRefresh
}

// Clear removes all cached keys.
func (c *JWKSCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = make(map[string]*jwkPublicKey)
}

// RefreshPeriodically starts a background goroutine that refreshes the JWKS cache
// at the given interval. It runs until the provided context is cancelled.
func (c *JWKSCache) RefreshPeriodically(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultJWKSTTL
	}

	refreshCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.refreshCtx = refreshCtx
	c.refreshCancel = cancel
	c.refreshing = true
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-refreshCtx.Done():
				c.mu.Lock()
				c.refreshing = false
				c.mu.Unlock()
				return
			case <-ticker.C:
				c.RefreshJWKS(refreshCtx, c.httpClient, c.jwksURL)
			}
		}
	}()
}

// RefreshJWKS fetches the JWKS from the well-known endpoint, parses all RSA
// public keys, and populates the cache. Safe to call concurrently.
func (c *JWKSCache) RefreshJWKS(ctx context.Context, httpClient *http.Client, issuerURL string) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	url := c.jwksURL
	if issuerURL != "" {
		url = issuerURL
	}

	// Ensure the URL points to the well-known JWKS endpoint
	if !strings.HasSuffix(url, "/.well-known/jwks.json") {
		if strings.HasSuffix(url, "/") {
			url += ".well-known/jwks.json"
		} else {
			url += "/.well-known/jwks.json"
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var rawKeys jwksResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(&rawKeys); err != nil {
		return
	}

	c.mu.Lock()
	c.keys = make(map[string]*jwkPublicKey)

	for _, rawKey := range rawKeys.Keys {
		var entry jwkEntry
		if err := json.Unmarshal(rawKey, &entry); err != nil {
			continue
		}

		// Only handle RSA keys (RS256/RS384/RS512 used by Authentik)
		if entry.KTY != "RSA" || entry.N == "" || entry.E == "" {
			continue
		}

		// Decode the RSA modulus (big-endian base64url-encoded)
		modBytes, err := base64.RawURLEncoding.DecodeString(entry.N)
		if err != nil {
			continue
		}

		// Decode the RSA exponent (big-endian base64url-encoded)
		expBytes, err := base64.RawURLEncoding.DecodeString(entry.E)
		if err != nil {
			continue
		}

		// Compute exponent from bytes
		exponent := 0
		for _, b := range expBytes {
			exponent = exponent*256 + int(b)
		}

		// Ensure modulus has a leading zero byte for positive big.Int
		if len(modBytes) > 0 && modBytes[0]&0x80 != 0 {
			modBytes = append([]byte{0}, modBytes...)
		}

		keyID := entry.KID
		if keyID == "" {
			keyID = "jwk-" + entry.N[:min(8, len(entry.N))]
		}

		c.keys[keyID] = &jwkPublicKey{
			Key: &rsa.PublicKey{
				N: new(big.Int).SetBytes(modBytes),
				E: exponent,
			},
			KeyID:     keyID,
			ExpiresAt: time.Now().Add(c.ttl),
		}
	}

	c.lastRefresh = time.Now()
	c.mu.Unlock()
}

// GetSigningKey retrieves the public key for RSA signing verification.
// Returns the key and true if found and not expired, or nil and false otherwise.
func (c *JWKSCache) GetSigningKey(keyID string) (*jwkPublicKey, bool) {
	c.mu.RLock()
	key, ok := c.keys[keyID]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if key.ExpiresAt.Before(time.Now()) {
		return nil, false
	}

	return key, true
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
