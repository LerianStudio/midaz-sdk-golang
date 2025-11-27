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

	// Prepare value for processing
	val = prepareReflectValue(val)
	if !val.IsValid() || val.Kind() != reflect.Struct {
		return result
	}

	processStructFields(result, val)

	return result
}

// prepareReflectValue handles pointer unwrapping and validation
func prepareReflectValue(val reflect.Value) reflect.Value {
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return reflect.Value{}
		}

		return val.Elem()
	}

	return val
}

// processStructFields iterates through struct fields and maps them
func processStructFields(result map[string]any, val reflect.Value) {
	t := val.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := val.Field(i)

		if shouldSkipField(field, value) {
			continue
		}

		fieldName := getFieldName(field)
		fieldValue := processFieldValue(value)

		if fieldValue != nil {
			result[fieldName] = fieldValue
		}
	}
}

// shouldSkipField determines if a field should be skipped during mapping
func shouldSkipField(field reflect.StructField, value reflect.Value) bool {
	// Skip unexported fields
	if !field.IsExported() {
		return true
	}

	// Skip nil pointers
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return true
	}

	return false
}

// getFieldName extracts the field name, considering JSON tags
func getFieldName(field reflect.StructField) string {
	name := field.Name

	if tag, ok := field.Tag.Lookup("json"); ok {
		if tagParts := parseTag(tag); tagParts[0] != "" && tagParts[0] != "-" {
			name = tagParts[0]
		}
	}

	return name
}

// processFieldValue processes a field value based on its type
func processFieldValue(value reflect.Value) any {
	// Handle time.Time special case
	if timeValue := processTimeValue(value); timeValue != nil {
		return timeValue
	}

	// Handle different value kinds
	switch value.Kind() {
	case reflect.Struct:
		return processStructValue(value)
	case reflect.Map:
		return processMapValue(value)
	case reflect.Slice:
		return processSliceValue(value)
	case reflect.Ptr:
		return processPointerValue(value)
	default:
		return value.Interface()
	}
}

// processTimeValue handles time.Time type conversion.
func processTimeValue(value reflect.Value) any {
	if !isTimeType(value) {
		return nil
	}

	timeValue, ok := extractTimeValue(value)
	if !ok {
		return nil
	}

	return ConvertToISODateTime(timeValue)
}

// extractTimeValue extracts time.Time from either a pointer or direct value.
func extractTimeValue(value reflect.Value) (time.Time, bool) {
	if value.Kind() == reflect.Ptr {
		t, ok := value.Elem().Interface().(time.Time)
		return t, ok
	}

	t, ok := value.Interface().(time.Time)

	return t, ok
}

// isTimeType checks if the value is a time.Time or *time.Time
func isTimeType(value reflect.Value) bool {
	timeType := reflect.TypeOf(time.Time{})

	return value.Type() == timeType ||
		(value.Kind() == reflect.Ptr && value.Elem().Type() == timeType)
}

// processStructValue handles nested struct mapping
func processStructValue(value reflect.Value) any {
	// Skip if it's a simple type like time.Time
	if value.Type() == reflect.TypeOf(time.Time{}) {
		return value.Interface()
	}

	return MapStruct(value.Interface())
}

// processMapValue handles map field values
func processMapValue(value reflect.Value) any {
	// Just copy the map as-is for now
	// (future enhancement: recursively process map values if they're structs)
	return value.Interface()
}

// processSliceValue handles slice field values
func processSliceValue(value reflect.Value) any {
	// Just copy the slice as-is for now
	// (future enhancement: recursively process slice elements if they're structs)
	return value.Interface()
}

// processPointerValue handles pointer field values
func processPointerValue(value reflect.Value) any {
	return value.Elem().Interface()
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
	unmapper := &structUnmapper[T]{
		target: target,
		data:   data,
	}
	unmapper.unmapToStruct()
}

// structUnmapper handles the unmapping of map data to struct fields.
type structUnmapper[T any] struct {
	target *T
	data   map[string]any
	elem   reflect.Value
	fields map[string]int
}

// unmapToStruct performs the main unmapping logic.
func (u *structUnmapper[T]) unmapToStruct() {
	if !u.validateTarget() {
		return
	}

	u.buildFieldMap()
	u.setFieldsFromData()
}

// validateTarget validates that the target is a pointer to a struct.
func (u *structUnmapper[T]) validateTarget() bool {
	val := reflect.ValueOf(u.target)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return false
	}

	u.elem = val.Elem()

	return true
}

// buildFieldMap creates a map of field names and JSON tags to field indexes.
func (u *structUnmapper[T]) buildFieldMap() {
	t := u.elem.Type()
	u.fields = make(map[string]int)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		u.addFieldMapping(field, i)
	}
}

// addFieldMapping adds field mappings by name and JSON tag.
func (u *structUnmapper[T]) addFieldMapping(field reflect.StructField, index int) {
	u.fields[field.Name] = index

	if tag, ok := field.Tag.Lookup("json"); ok {
		if tagParts := parseTag(tag); tagParts[0] != "" && tagParts[0] != "-" {
			u.fields[tagParts[0]] = index
		}
	}
}

// setFieldsFromData sets struct fields from map values.
func (u *structUnmapper[T]) setFieldsFromData() {
	for key, value := range u.data {
		if fieldIndex, ok := u.fields[key]; ok {
			u.setField(fieldIndex, value)
		}
	}
}

// setField sets a single field value.
func (u *structUnmapper[T]) setField(fieldIndex int, value any) {
	field := u.elem.Field(fieldIndex)
	if !field.CanSet() || value == nil {
		return
	}

	fieldType := field.Type()
	val := reflect.ValueOf(value)

	if u.handleTimeField(field, fieldType, value) {
		return
	}

	if u.handlePointerField(field, fieldType, val) {
		return
	}

	u.handleDirectAssignment(field, fieldType, val)
}

// handleTimeField handles time.Time and *time.Time field assignments.
func (*structUnmapper[T]) handleTimeField(field reflect.Value, fieldType reflect.Type, value any) bool {
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.String {
		return false
	}

	timeStr, ok := value.(string)
	if !ok {
		return false
	}

	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return false
	}

	// Handle direct time.Time fields
	if fieldType == reflect.TypeOf(time.Time{}) {
		field.Set(reflect.ValueOf(parsedTime))
		return true
	}

	// Handle *time.Time fields
	if fieldType.Kind() == reflect.Ptr && fieldType.Elem() == reflect.TypeOf(time.Time{}) {
		field.Set(reflect.ValueOf(&parsedTime))
		return true
	}

	return false
}

// handlePointerField handles pointer type field assignments.
func (u *structUnmapper[T]) handlePointerField(field reflect.Value, fieldType reflect.Type, val reflect.Value) bool {
	if fieldType.Kind() != reflect.Ptr || val.Kind() == reflect.Ptr {
		return false
	}

	newVal := reflect.New(fieldType.Elem())
	if u.assignValue(newVal.Elem(), fieldType.Elem(), val) {
		field.Set(newVal)
	}

	return true
}

// handleDirectAssignment handles direct field assignments.
func (u *structUnmapper[T]) handleDirectAssignment(field reflect.Value, fieldType reflect.Type, val reflect.Value) {
	u.assignValue(field, fieldType, val)
}

// assignValue attempts to assign a value to a field, handling type conversion.
func (*structUnmapper[T]) assignValue(field reflect.Value, fieldType reflect.Type, val reflect.Value) bool {
	if val.Type().AssignableTo(fieldType) {
		field.Set(val)
		return true
	}

	if val.Type().ConvertibleTo(fieldType) {
		field.Set(val.Convert(fieldType))
		return true
	}

	return false
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
