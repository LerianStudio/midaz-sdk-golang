package conversion

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test struct for conversion tests
type TestPerson struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Age        int         `json:"age"`
	Email      *string     `json:"email,omitempty"`
	Active     bool        `json:"active"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  *time.Time  `json:"updated_at,omitempty"`
	Address    TestAddress `json:"address,omitempty"` // Not a pointer to test direct struct-to-map conversion
	Tags       []string    `json:"tags,omitempty"`
	unexported string      // This field should be ignored
}

type TestAddress struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	Country string `json:"country"`
}

func TestMapStruct(t *testing.T) {
	// Create a test time
	now := time.Now().UTC()
	updated := now.Add(24 * time.Hour)

	// Create test data
	email := "test@example.com"
	person := TestPerson{
		ID:        "123",
		Name:      "John Doe",
		Age:       30,
		Email:     &email,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: &updated,
		Address: TestAddress{
			Street:  "123 Main St",
			City:    "Anytown",
			Country: "US",
		},
		Tags:       []string{"customer", "active"},
		unexported: "ignored",
	}

	// Convert to map
	result := MapStruct(person)

	// Assertions
	assert.Equal(t, "123", result["id"])
	assert.Equal(t, "John Doe", result["name"])
	assert.Equal(t, 30, result["age"])
	assert.Equal(t, email, result["email"])
	assert.Equal(t, true, result["active"])
	assert.Equal(t, ConvertToISODateTime(now), result["created_at"])
	assert.Equal(t, ConvertToISODateTime(updated), result["updated_at"])

	// Check that address was included as a map
	addressMap, ok := result["address"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "123 Main St", addressMap["street"])

	// Check that tags were included
	tags, ok := result["tags"].([]string)
	assert.True(t, ok)
	assert.Contains(t, tags, "customer")

	// Check that unexported field was not included
	_, hasUnexported := result["unexported"]
	assert.False(t, hasUnexported)
}

func TestUnmapStruct(t *testing.T) {
	// Create test map
	now := time.Now().UTC()
	data := map[string]any{
		"id":         "456",
		"name":       "Jane Smith",
		"age":        35,
		"email":      "jane@example.com",
		"active":     false,
		"created_at": ConvertToISODateTime(now),
		"address": map[string]any{
			"street":  "456 Oak Ave",
			"city":    "Othertown",
			"country": "CA",
		},
		"tags": []string{"supplier", "new"},
	}

	// Convert to struct
	var person TestPerson

	UnmapStruct(data, &person)

	// Assertions
	assert.Equal(t, "456", person.ID)
	assert.Equal(t, "Jane Smith", person.Name)
	assert.Equal(t, 35, person.Age)
	assert.NotNil(t, person.Email)
	assert.Equal(t, "jane@example.com", *person.Email)
	assert.False(t, person.Active)
	assert.Equal(t, now.Year(), person.CreatedAt.Year())
	assert.Equal(t, now.Month(), person.CreatedAt.Month())
	assert.Equal(t, now.Day(), person.CreatedAt.Day())

	// For structs, we expect an empty struct rather than nil
	assert.Equal(t, TestAddress{}, person.Address)
}

func TestMapSlice(t *testing.T) {
	// Test data
	input := []string{"one", "two", "three"}

	// Apply mapping function
	result := MapSlice(input, func(s string) int {
		return len(s)
	})

	// Assertions
	assert.Equal(t, []int{3, 3, 5}, result)

	// Test with struct mapping
	people := []TestPerson{
		{ID: "1", Name: "Alice"},
		{ID: "2", Name: "Bob"},
	}

	// Map to simple struct
	type SimpleUser struct {
		ID   string
		Name string
	}

	users := MapSlice(people, func(p TestPerson) SimpleUser {
		return SimpleUser{ID: p.ID, Name: p.Name}
	})

	assert.Len(t, users, 2)
	assert.Equal(t, "Alice", users[0].Name)
}

func TestFilterSlice(t *testing.T) {
	// Test data
	input := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Filter even numbers
	result := FilterSlice(input, func(i int) bool {
		return i%2 == 0
	})

	// Assertions
	assert.Equal(t, []int{2, 4, 6, 8, 10}, result)

	// Test with struct filtering
	people := []TestPerson{
		{ID: "1", Name: "Alice", Age: 20},
		{ID: "2", Name: "Bob", Age: 30},
		{ID: "3", Name: "Charlie", Age: 40},
	}

	// Filter people over 25
	filtered := FilterSlice(people, func(p TestPerson) bool {
		return p.Age > 25
	})

	assert.Len(t, filtered, 2)
	assert.Equal(t, "Bob", filtered[0].Name)
	assert.Equal(t, "Charlie", filtered[1].Name)
}

func TestReduceSlice(t *testing.T) {
	// Test data
	input := []int{1, 2, 3, 4, 5}

	// Sum all numbers
	sum := ReduceSlice(input, 0, func(acc int, i int) int {
		return acc + i
	})

	// Assertions
	assert.Equal(t, 15, sum)

	// Test with struct reduction
	people := []TestPerson{
		{ID: "1", Name: "Alice", Age: 20},
		{ID: "2", Name: "Bob", Age: 30},
		{ID: "3", Name: "Charlie", Age: 40},
	}

	// Calculate average age
	totalAge := ReduceSlice(people, 0, func(acc int, p TestPerson) int {
		return acc + p.Age
	})

	avgAge := totalAge / len(people)
	assert.Equal(t, 30, avgAge)
}

func TestPtrValue(t *testing.T) {
	// Test with nil pointer
	var nilPtr *string

	result := PtrValue(nilPtr, "default")
	assert.Equal(t, "default", result)

	// Test with non-nil pointer
	value := "actual"
	ptr := &value
	result = PtrValue(ptr, "default")
	assert.Equal(t, "actual", result)
}

func TestToPtr(t *testing.T) {
	// Test simple value
	ptr := ToPtr("test")
	assert.NotNil(t, ptr)
	assert.Equal(t, "test", *ptr)

	// Test integer
	intPtr := ToPtr(42)
	assert.NotNil(t, intPtr)
	assert.Equal(t, 42, *intPtr)

	// Test boolean
	boolPtr := ToPtr(true)
	assert.NotNil(t, boolPtr)
	assert.True(t, *boolPtr)
}
