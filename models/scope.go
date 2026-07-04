// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// Scope narrows the set of transactions a tracer rule or limit applies to. Every
// field is optional; a nil field means "matches any value for this dimension".
// An empty Scope (all fields nil) matches every transaction.
//
// Scope is shared by both tracer resources: rules (models.Rule.Scopes) and
// limits (models.Limit.Scopes). The wire shape mirrors the generated
// gentracer.Scope one-for-one — camelCase json tags, all fields omitempty — so a
// server-issued scope round-trips through this struct without loss.
//
// The ID fields (AccountID/MerchantID/PortfolioID/SegmentID) are UUID strings on
// the wire. They are modeled as *string rather than a UUID type so the SDK
// surface stays free of the generated openapi_types.UUID and callers pass plain
// strings.
type Scope struct {
	// AccountID scopes to a single account (UUID).
	AccountID *string `json:"accountId,omitempty"`

	// MerchantID scopes to a single merchant (UUID).
	MerchantID *string `json:"merchantId,omitempty"`

	// PortfolioID scopes to a single portfolio (UUID).
	PortfolioID *string `json:"portfolioId,omitempty"`

	// SegmentID scopes to a single segment (UUID).
	SegmentID *string `json:"segmentId,omitempty"`

	// SubType scopes to a transaction sub-type (free-form, max 50 chars).
	SubType *string `json:"subType,omitempty"`

	// TransactionType scopes to a transaction type (e.g. CARD, WIRE, PIX, CRYPTO).
	TransactionType *string `json:"transactionType,omitempty"`
}
