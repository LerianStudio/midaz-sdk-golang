// Package serverversion parses the Midaz ledger /version body and decides who applies fees.
// An uncertain version resolves to FeeModeLegacy, since /v1 never applies fees on v3 or v4:
// legacy charges a fee only through the caller's own fee engine, never through the ledger.
package serverversion
