package store

import "time"

// Channel represents a row in the channels table.
type Channel struct {
	ID          string         `json:"id" db:"id" dbjson:"id"`
	TenantID    string         `json:"tenant_id" db:"tenant_id"`
	Name        string         `json:"name" db:"name"`
	Description *string        `json:"description,omitempty" db:"description"`
	ChannelType string         `json:"channel_type" db:"channel_type"`
	CreatorID   string         `json:"creator_id" db:"creator_id"`
	MaxMembers  int            `json:"max_members" db:"max_members"`
	IsPublic    bool           `json:"is_public" db:"is_public"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}

// ChannelMember represents a row in the channel_members table.
type ChannelMember struct {
	ID        string    `json:"id" db:"id" dbjson:"id"`
	ChannelID string    `json:"channel_id" db:"channel_id"`
	AgentID   string    `json:"agent_id" db:"agent_id"`
	Role      string    `json:"role" db:"role"`
	JoinedAt  time.Time `json:"joined_at" db:"joined_at"`
}

// Message represents a row in the messages table.
type Message struct {
	ID           string                 `json:"id" db:"id" dbjson:"id"`
	TenantID     string                 `json:"tenant_id" db:"tenant_id"`
	ChannelID    string                 `json:"channel_id" db:"channel_id"`
	ParentID     *string                `json:"parent_id,omitempty" db:"parent_id"`
	SenderID     string                 `json:"sender_id" db:"sender_id"`
	SenderName   *string                `json:"sender_name,omitempty" db:"sender_name"`
	MessageType  string                 `json:"message_type" db:"message_type"`
	Content      string                 `json:"content" db:"content"`
	Attachments  []map[string]interface{} `json:"attachments,omitempty" db:"attachments"`
	ReplyCount   int                    `json:"reply_count" db:"reply_count"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
}

// Handoff represents a row in the handoffs table.
type Handoff struct {
	ID             string                 `json:"id" db:"id" dbjson:"id"`
	TenantID       string                 `json:"tenant_id" db:"tenant_id"`
	FromAgentID    string                 `json:"from_agent_id" db:"from_agent_id"`
	ToAgentID      string                 `json:"to_agent_id" db:"to_agent_id"`
	ChannelID      *string                `json:"channel_id,omitempty" db:"channel_id"`
	ParentMessageID *string               `json:"parent_message_id,omitempty" db:"parent_message_id"`
	Title          string                 `json:"title" db:"title"`
	Description    *string                `json:"description,omitempty" db:"description"`
	Context        map[string]interface{} `json:"context" db:"context"`
	Priority       string                 `json:"priority" db:"priority"`
	Status         string                 `json:"status" db:"status"`
	ExpiresAt      *time.Time             `json:"expires_at,omitempty" db:"expires_at"`
	AssignedAt     *time.Time             `json:"assigned_at,omitempty" db:"assigned_at"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	Response       *string                `json:"response,omitempty" db:"response"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" db:"updated_at"`
}

// Presence represents a row in the presence table.
type Presence struct {
	ID             string                 `json:"id" db:"id" dbjson:"id"`
	TenantID       string                 `json:"tenant_id" db:"tenant_id"`
	AgentID        string                 `json:"agent_id" db:"agent_id"`
	Status         string                 `json:"status" db:"status"`
	LastHeartbeat  time.Time              `json:"last_heartbeat" db:"last_heartbeat"`
	Metadata       map[string]interface{} `json:"metadata" db:"metadata"`
}