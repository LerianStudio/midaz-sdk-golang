// Package conversion provides utilities for converting between different data formats
// and creating human-readable representations of Midaz SDK models.
package conversion

import (
	"reflect"
	"time"
)

// MapStruct converts a struct to a map using field names (or json tags if available) as keys
// This generic utility reduces the need for custom struct-to-map conversions
//
// Example:
//
//	type User struct {
//	    ID      string `json:"id"`
//	    Name    string `json:"name"`
//	    Created time.Time `json:"created_at"`
//	}
//
//	user := User{ID: "123", Name: "John", Created: time.Now()}
//	userMap := conversion.MapStruct(user)
//	// Result: map[string]any{"id": "123", "name": "John", "created_at": "2025-04-01T12:34:56Z"}
func MapStruct[T any](data T) map[string]any {
	result := make(map[string]any)
	val := reflect.ValueOf(data)

	// Handle pointer input
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return result
		}
		val = val.Elem()
	}

	// Only process structs
	if val.Kind() != reflect.Struct {
		return result
	}

	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get the field name, checking for json tag
		name := field.Name
		if tag, ok := field.Tag.Lookup("json"); ok {
			// Parse the json tag (handling omitempty, etc.)
			if tagParts := parseTag(tag); tagParts[0] != "" && tagParts[0] != "-" {
				name = tagParts[0]
			}
		}

		// Handle nil pointers
		if value.Kind() == reflect.Ptr && value.IsNil() {
			continue
		}

		// Handle time.Time by converting to ISO format
		if value.Type() == reflect.TypeOf(time.Time{}) ||
			(value.Kind() == reflect.Ptr && value.Elem().Type() == reflect.TypeOf(time.Time{})) {
			var timeValue time.Time
			if value.Kind() == reflect.Ptr {
				timeValue = value.Elem().Interface().(time.Time)
			} else {
				timeValue = value.Interface().(time.Time)
			}
			result[name] = ConvertToISODateTime(timeValue)
			continue
		}

		// Handle nested structs by recursively mapping them
		if value.Kind() == reflect.Struct {
			// Skip if it's a simple type like time.Time
			if value.Type() == reflect.TypeOf(time.Time{}) {
				result[name] = value.Interface()
			} else {
				nestedMap := MapStruct(value.Interface())
				result[name] = nestedMap
			}
			continue
		}

		// Handle maps of structs
		if value.Kind() == reflect.Map {
			// Just copy the map as-is for now
			// (future enhancement: recursively process map values if they're structs)
			result[name] = value.Interface()
			continue
		}

		// Handle slices
		if value.Kind() == reflect.Slice {
			// Just copy the slice as-is for now
			// (future enhancement: recursively process slice elements if they're structs)
			result[name] = value.Interface()
			continue
		}

		// Handle standard types
		if value.Kind() == reflect.Ptr {
			result[name] = value.Elem().Interface()
		} else {
			result[name] = value.Interface()
		}
	}

	return result
}

// UnmapStruct converts a map to a struct, using field names or json tags
// This generic utility reduces the need for custom map-to-struct conversions
//
// Example:
//
//	type User struct {
//	    ID      string `json:"id"`
//	    Name    string `json:"name"`
//	    Created time.Time `json:"created_at"`
//	}
//
//	userMap := map[string]any{"id": "123", "name": "John", "created_at": "2025-04-01T12:34:56Z"}
//	user := User{}
//	conversion.UnmapStruct(userMap, &user)
func UnmapStruct[T any](data map[string]any, target *T) {
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return
	}

	elem := val.Elem()
	t := elem.Type()

	// Build a map of struct fields by json tag and by field name
	fields := make(map[string]int)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Add to field map by field name
		fields[field.Name] = i

		// Add to field map by json tag
		if tag, ok := field.Tag.Lookup("json"); ok {
			if tagParts := parseTag(tag); tagParts[0] != "" && tagParts[0] != "-" {
				fields[tagParts[0]] = i
			}
		}
	}

	// Set struct fields from map values
	for key, value := range data {
		if fieldIndex, ok := fields[key]; ok {
			field := elem.Field(fieldIndex)
			if !field.CanSet() {
				continue
			}

			fieldType := field.Type()
			val := reflect.ValueOf(value)

			// Handle nil values
			if value == nil {
				continue
			}

			// Handle time.Time fields
			if fieldType == reflect.TypeOf(time.Time{}) && val.Kind() == reflect.String {
				timeStr := value.(string)
				parsedTime, err := time.Parse(time.RFC3339, timeStr)
				if err == nil {
					field.Set(reflect.ValueOf(parsedTime))
				}
				continue
			}

			// Handle pointers to time.Time
			if fieldType.Kind() == reflect.Ptr && fieldType.Elem() == reflect.TypeOf(time.Time{}) && val.Kind() == reflect.String {
				timeStr := value.(string)
				parsedTime, err := time.Parse(time.RFC3339, timeStr)
				if err == nil {
					field.Set(reflect.ValueOf(&parsedTime))
				}
				continue
			}

			// Handle other pointer types
			if fieldType.Kind() == reflect.Ptr && val.Kind() != reflect.Ptr {
				// Create a new pointer of the appropriate type
				newVal := reflect.New(fieldType.Elem())

				// Set the pointed-to value, handling different types
				if val.Type().AssignableTo(fieldType.Elem()) {
					newVal.Elem().Set(val)
					field.Set(newVal)
				} else if val.Type().ConvertibleTo(fieldType.Elem()) {
					newVal.Elem().Set(val.Convert(fieldType.Elem()))
					field.Set(newVal)
				}
				continue
			}

			// Handle direct assignment
			if val.Type().AssignableTo(fieldType) {
				field.Set(val)
			} else if val.Type().ConvertibleTo(fieldType) {
				field.Set(val.Convert(fieldType))
			}
		}
	}
}

// MapSlice applies the mapping function to each element of the input slice
// and returns a new slice with the mapped elements
//
// Example:
//
//	type User struct {
//	    ID   string
//	    Name string
//	}
//
//	type UserDTO struct {
//	    ID   string
//	    Name string
//	}
//
//	users := []User{
//	    {ID: "1", Name: "Alice"},
//	    {ID: "2", Name: "Bob"},
//	}
//
//	// Map User to UserDTO
//	userDTOs := conversion.MapSlice(users, func(u User) UserDTO {
//	    return UserDTO{ID: u.ID, Name: u.Name}
//	})
func MapSlice[T any, R any](slice []T, mapFn func(T) R) []R {
	result := make([]R, len(slice))
	for i, item := range slice {
		result[i] = mapFn(item)
	}
	return result
}

// FilterSlice filters a slice based on a predicate function
//
// Example:
//
//	users := []User{
//	    {ID: "1", Name: "Alice", Age: 30},
//	    {ID: "2", Name: "Bob", Age: 25},
//	    {ID: "3", Name: "Charlie", Age: 35},
//	}
//
//	// Filter users over 30
//	olderUsers := conversion.FilterSlice(users, func(u User) bool {
//	    return u.Age >= 30
//	})
func FilterSlice[T any](slice []T, filterFn func(T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if filterFn(item) {
			result = append(result, item)
		}
	}
	return result
}

// ReduceSlice reduces a slice to a single value using a reducer function
//
// Example:
//
//	transactions := []Transaction{
//	    {Amount: 100},
//	    {Amount: 200},
//	    {Amount: 300},
//	}
//
//	// Sum all transaction amounts
//	total := conversion.ReduceSlice(transactions, 0, func(acc int, tx Transaction) int {
//	    return acc + tx.Amount
//	})
func ReduceSlice[T any, R any](slice []T, initial R, reduceFn func(R, T) R) R {
	result := initial
	for _, item := range slice {
		result = reduceFn(result, item)
	}
	return result
}

// PtrValue returns the value of a pointer, or default value if the pointer is nil
// This is a helper to safely handle nil pointers
//
// Example:
//
//	name := PtrValue(user.Name, "Anonymous")
//	// If user.Name is nil, "Anonymous" is returned, otherwise the value pointed to
func PtrValue[T any](ptr *T, defaultValue T) T {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// ToPtr creates a pointer to the given value
// This is a helper to easily create pointers to constants or literals
//
// Example:
//
//	// Create a pointer to a string literal
//	namePtr := conversion.ToPtr("John")
//
//	// Create a pointer to a constant
//	const status = "active"
//	statusPtr := conversion.ToPtr(status)
func ToPtr[T any](value T) *T {
	return &value
}

// parseTag parses a tag string into its components
// This is an internal helper function for processing struct tags
func parseTag(tag string) []string {
	// Simple implementation for json tags
	for i, c := range tag {
		if c == ',' || c == ' ' {
			return []string{tag[:i], tag[i+1:]}
		}
	}
	return []string{tag, ""}
}
