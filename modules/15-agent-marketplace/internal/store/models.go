package store

import "time"

// Listing represents a marketplace listing (agent, template, tool, integration, or skill).
type Listing struct {
	ID                    string         `json:"id"`
	VendorID              string         `json:"vendor_id"`
	Name                  string         `json:"name"`
	Description           string         `json:"description"`
	Category              string         `json:"category"`               // agent, template, tool, integration, skill
	ListingType           string         `json:"listing_type"`           // vetted, user_generated
	Status                string         `json:"status"`                 // draft, pending_review, approved, rejected, deactivated
	Version               string         `json:"version"`
	CompatibilityVersions JSONB          `json:"compatibility_versions"` // {"m04_min": "1.0", "m03_min": "1.0"}
	Capabilities          StringArray    `json:"capabilities"`
	SupportedLanguages    StringArray    `json:"supported_languages"`
	RequiresSubscription  bool           `json:"requires_subscription"`
	SubscriptionTier      string         `json:"subscription_tier"`      // free, trial, basic, pro, enterprise
	TrialDays             int            `json:"trial_days"`
	PriceUSD              float64        `json:"price_usd"`
	RatingAvg             float64        `json:"rating_avg"`
	RatingCount           int            `json:"rating_count"`
	DownloadCount         int            `json:"download_count"`
	Metadata              JSONB          `json:"metadata"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

// TenantSubscription represents a tenant's subscription to a marketplace listing.
type TenantSubscription struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	ListingID      string    `json:"listing_id"`
	Status         string    `json:"status"`        // active, expired, cancelled, trial
	StartedAt      time.Time `json:"started_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	AutoRenew    bool      `json:"auto_renew"`
	SubscriptionTier string    `json:"subscription_tier"`
	TrialUsed      bool      `json:"trial_used"`
	Deployed       bool      `json:"deployed"`
	DeployedAt     *time.Time `json:"deployed_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListingWithSub checks if the tenant has an active subscription.
type ListingWithSub struct {
	Listing        *Listing  `json:"listing"`
	Subscribed     bool      `json:"subscribed"`
	SubscriptionID *string   `json:"subscription_id,omitempty"`
}

// Review represents a tenant's review of a marketplace listing.
type Review struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	ListingID       string    `json:"listing_id"`
	Rating          int       `json:"rating"`
	Title           string    `json:"title"`
	ReviewText      string    `json:"review_text"`
	VerifiedPurchase bool     `json:"verified_purchase"`
	HelpfulCount    int       `json:"helpful_count"`
	Status          string    `json:"status"` // active, flagged, removed
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ListingFilter holds query parameters for listing search.
type ListingFilter struct {
	Page               int
	PageSize           int
	Category           string
	ListingType        string
	Status             string
	RequiresSubscription *bool
	SubscriptionTier   string
	Capability         string
	Language           string
	PriceMin           *float64
	PriceMax           *float64
	RatingMin          *float64
	Search             string
}

// PaginatedListings holds paginated listing results.
type PaginatedListings struct {
	Listings []Listing `json:"listings"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Total    int       `json:"total"`
}

// PaginatedSubscriptions holds paginated subscription results.
type PaginatedSubscriptions struct {
	Subscriptions []TenantSubscription `json:"subscriptions"`
	Page          int                  `json:"page"`
	PageSize      int                  `json:"page_size"`
	Total         int                  `json:"total"`
}

// PaginatedReviews holds paginated review results.
type PaginatedReviews struct {
	Reviews []Review `json:"reviews"`
	Page    int      `json:"page"`
	PageSize int     `json:"page_size"`
	Total   int      `json:"total"`
}

// JSONB is a simple JSONB wrapper that handles null JSONB columns.
type JSONB struct {
	String string
	Valid  bool
}

// Scan scans a value into JSONB.
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		j.String = ""
		j.Valid = false
		return nil
	}
	switch v := value.(type) {
	case []byte:
		j.String = string(v)
	case string:
		j.String = v
	default:
		j.String = ""
	}
	j.Valid = true
	return nil
}

// Value returns the JSONB value for database writing.
func (j JSONB) Value() (interface{}, error) {
	if !j.Valid {
		return nil, nil
	}
	return j.String, nil
}

// StringArray is a string array wrapper for PostgreSQL TEXT[].
type StringArray struct {
	String string
	Valid  bool
}

// Scan scans a value into StringArray.
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		s.String = "[]"
		s.Valid = false
		return nil
	}
	switch v := value.(type) {
	case []byte:
		s.String = string(v)
	case string:
		s.String = v
	default:
		s.String = "{}"
	}
	s.Valid = true
	return nil
}

// Value returns the StringArray value for database writing.
func (s StringArray) Value() (interface{}, error) {
	if !s.Valid {
		return nil, nil
	}
	return s.String, nil
}

// ToSlice returns the parsed string slice.
func (s StringArray) ToSlice() []string {
	if !s.Valid {
		return []string{}
	}
	return []string{s.String}
}