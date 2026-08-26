// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// This file holds the SHARED context models the tracer uses to describe the
// parties to a transaction. They are the input side of a validation
// (models.ValidateTransactionInput) and the output side of a stored validation
// record (models.TransactionValidation), and Epic 4.2's reservation facade
// reuses them verbatim — hence a standalone file, following the models/scope.go
// precedent (a shared value type used by more than one tracer resource).
//
// Every wire shape mirrors the generated gentracer.*Context one-for-one:
// camelCase json tags, and the ID fields are UUID strings on the wire modeled as
// plain string so the SDK surface never leaks the generated openapi_types.UUID.

// AccountContext is the account party to a transaction. AccountID, Status, and
// Type are required by the server; Metadata is optional free-form context.
type AccountContext struct {
	// AccountID is the account UUID.
	AccountID string `json:"accountId"`

	// Status is the account status (server-defined).
	Status string `json:"status"`

	// Type is the account type (server-defined).
	Type string `json:"type"`

	// Metadata is optional free-form context attached to the account.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SegmentContext is the optional segment party to a transaction. SegmentID is
// required when a segment context is supplied; Name and Metadata are optional.
type SegmentContext struct {
	// SegmentID is the segment UUID.
	SegmentID string `json:"segmentId"`

	// Name is an optional human-readable segment name.
	Name *string `json:"name,omitempty"`

	// Metadata is optional free-form context attached to the segment.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PortfolioContext is the optional portfolio party to a transaction. PortfolioID
// is required when a portfolio context is supplied; Name and Metadata are
// optional.
type PortfolioContext struct {
	// PortfolioID is the portfolio UUID.
	PortfolioID string `json:"portfolioId"`

	// Name is an optional human-readable portfolio name.
	Name *string `json:"name,omitempty"`

	// Metadata is optional free-form context attached to the portfolio.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MerchantContext is the optional merchant party to a transaction. MerchantID,
// Category, Country, and Name are required when a merchant context is supplied;
// Metadata is optional.
type MerchantContext struct {
	// MerchantID is the merchant UUID.
	MerchantID string `json:"merchantId"`

	// Category is the merchant category code (server-defined).
	Category string `json:"category"`

	// Country is the merchant country (ISO code, server-defined).
	Country string `json:"country"`

	// Name is the human-readable merchant name.
	Name string `json:"name"`

	// Metadata is optional free-form context attached to the merchant.
	Metadata map[string]any `json:"metadata,omitempty"`
}
