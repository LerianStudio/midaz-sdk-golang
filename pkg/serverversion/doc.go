// Package serverversion parses the Midaz ledger /version body and decides who applies fees.
// An uncertain version always resolves to FeeModeLegacy, which is correct on both v3 and v4:
// /v1 never applies fees on either, so the external fee engine keeps charging exactly once.
package serverversion
