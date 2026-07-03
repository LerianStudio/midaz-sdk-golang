// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import "net/http"

// MockHTTPClient is a shared test double for the SDK's HTTP client. It was
// extracted from the deleted accounts_test.go during Epic 5.4 (legacy resource
// delete) because the surviving trio tests (balances/operations) depend on it
// and its mockTransport adapter.
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

type mockTransport struct {
	mock *MockHTTPClient
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.mock.DoFunc(req)
}
