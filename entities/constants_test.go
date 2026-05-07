package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConstants pins the wire values of HTTP header constants that ride on
// every request. Test exists to guarantee a rename does not silently change
// the wire shape — header names are NOT free to evolve.
func TestConstants(t *testing.T) {
	assert.Equal(t, "X-Total-Count", HeaderTotalCount)
	assert.Equal(t, "true", BoolTrue)
}
