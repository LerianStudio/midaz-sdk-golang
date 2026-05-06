// Package models defines the SDK-owned data models for the Midaz API.
//
// Every type in this package is hand-written and self-contained. JSON
// tags align with the wire shape Midaz expects, so structs marshal
// directly to/from request and response payloads with no adapter or
// conversion layer between the SDK and the backend.
//
// Key model types:
//
// Account: an account in the Midaz system — the fundamental entity
// for tracking assets and balances. Accounts belong to organizations
// and ledgers.
//
// Asset: a unit of value that can be tracked and transferred — fiat
// currency, security, or other financial instrument.
//
// Balance: the current state of an account's holdings for a specific
// asset, including total, available, and on-hold amounts.
//
// Ledger: a collection of accounts and transactions within an
// organization, providing the complete record of financial activity.
//
// Organization: a business entity that owns ledgers, accounts, and
// other resources within the Midaz system.
//
// Portfolio: a collection of accounts belonging to a specific entity
// within an organization and ledger, used for grouping and management.
//
// Segment: a categorization unit for more granular organization of
// accounts or other entities within a ledger.
//
// Transaction: a financial event that affects one or more accounts
// through a series of operations (debits and credits).
//
// Operation: an individual accounting entry within a transaction —
// typically a debit or credit to a specific account.
//
// Each type ships with constructors, fluent With* builders for optional
// fields, and dedicated Create*Input / Update*Input shapes for the
// request side of each resource.
package models

// Note: This file serves as documentation for the model package architecture.
// The actual models are defined in their respective files.
