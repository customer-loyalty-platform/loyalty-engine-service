package model

import (
	"encoding/json"
	"time"
)

type MemberBalance struct {
	MemberID        string    `json:"member_id"`
	TotalPoints     float64   `json:"total_points"`
	LockedPoints    float64   `json:"locked_points"`
	ExpiringPoints  float64   `json:"expiring_points"`
	AvailablePoints float64   `json:"available_points"`
	Version         int       `json:"version"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PointLedgerRow struct {
	ID             string    `json:"id"`
	MemberID       string    `json:"member_id"`
	Type           string    `json:"type"`
	Amount         float64   `json:"amount"`
	BalanceAfter   float64   `json:"balance_after"`
	ReferenceID    string    `json:"reference_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}

type ExpiringBucketRow struct {
	ID              string    `json:"id"`
	MemberID        string    `json:"member_id"`
	OriginalPoints  float64   `json:"original_points"`
	RemainingPoints float64   `json:"remaining_points"`
	EarnedAt        time.Time `json:"earned_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	ExpiryPolicy    string    `json:"expiry_policy"`
	Status          string    `json:"status"`
	DaysRemaining   int       `json:"days_remaining"`
}

type TierInfo struct {
	TierID     string          `json:"tier_id"`
	TierCode   string          `json:"tier_code"`
	TierName   string          `json:"tier_name"`
	Multiplier float64         `json:"multiplier"`
	ColorHex   string          `json:"color_hex,omitempty"`
	IconURL    string          `json:"icon_url,omitempty"`
	Perks      json.RawMessage `json:"perks,omitempty"`
}

type MemberProfile struct {
	MemberID      string         `json:"member_id"`
	AccountNumber string         `json:"account_number"`
	Status        string         `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	Tier          *TierInfo      `json:"tier,omitempty"`
	Balance       *MemberBalance `json:"balance,omitempty"`
}

type RegisterMemberRequest struct {
	AccountNumber string `json:"account_number" binding:"required"`
	TierCode      string `json:"tier_code"` // Optional: default to lowest sort order tier
}

type RegisterMemberResponse struct {
	MemberID      string    `json:"member_id"`
	AccountNumber string    `json:"account_number"`
	Status        string    `json:"status"`
	TierCode      string    `json:"tier_code"`
	CreatedAt     time.Time `json:"created_at"`
}

