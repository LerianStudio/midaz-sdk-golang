package conversion

import (
	"reflect"
	"strings"
	"time"
)

// ModelConverter provides generic conversion between SDK models and backend models
// using reflection to automatically map fields with the same name
//
// Example:
//
//	sdkModel := &models.Account{ID: "123", Name: "Test Account"}
//	backendModel := &mmodel.Account{}
//	err := conversion.ModelConverter(sdkModel, backendModel)
//
// This function attempts to map fields with the same name while preserving types.
// It can handle basic field types, pointers, maps, and slices of these types.
//
// For more complex conversions, you should implement custom mappers instead.
func ModelConverter(source, target any) error {
	sourceVal, targetVal, ok := dereferencePointers(source, target)
	if !ok {
		return nil // nothing to convert or nowhere to store
	}

	// Both source and target must be structs
	if !areStructs(&sourceVal, &targetVal) {
		return nil
	}

	targetFields := buildTargetFieldMap(targetVal)
	copyMatchingFields(sourceVal, targetVal, targetFields)

	return nil
}

// dereferencePointers navigates through pointers to get the actual values
func dereferencePointers(source, target any) (sourceVal reflect.Value, targetVal reflect.Value, ok bool) {
	sourceVal = reflect.ValueOf(source)
	targetVal = reflect.ValueOf(target)

	var (
		sourceOk bool
		targetOk bool
	)

	sourceVal, sourceOk = dereferenceValue(sourceVal)
	targetVal, targetOk = dereferenceValue(targetVal)

	return sourceVal, targetVal, sourceOk && targetOk
}

// dereferenceValue navigates through a single pointer
func dereferenceValue(val reflect.Value) (reflect.Value, bool) {
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return reflect.Value{}, false
		}

		val = val.Elem()
	}

	return val, true
}

// areStructs checks if both values are structs
func areStructs(sourceVal, targetVal *reflect.Value) bool {
	return sourceVal.Kind() == reflect.Struct && targetVal.Kind() == reflect.Struct
}

// buildTargetFieldMap creates a map of target field names for faster lookup
func buildTargetFieldMap(targetVal reflect.Value) map[string]reflect.Value {
	targetFields := make(map[string]reflect.Value)
	targetType := targetVal.Type()

	for i := 0; i < targetVal.NumField(); i++ {
		field := targetType.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldValue := targetVal.Field(i)
		addFieldToMap(targetFields, field, fieldValue)
	}

	return targetFields
}

// addFieldToMap adds a field to the target field map with appropriate names
func addFieldToMap(targetFields map[string]reflect.Value, field reflect.StructField, fieldValue reflect.Value) {
	// Use lowercase field name for case-insensitive matching
	targetFields[strings.ToLower(field.Name)] = fieldValue

	// Also check JSON tag if present
	if tag, ok := field.Tag.Lookup("json"); ok {
		tagName := extractJSONTagName(tag)
		if tagName != "" {
			targetFields[strings.ToLower(tagName)] = fieldValue
		}
	}
}

// extractJSONTagName extracts the field name from a JSON tag
func extractJSONTagName(tag string) string {
	tagParts := strings.Split(tag, ",")
	if tagParts[0] != "" && tagParts[0] != "-" {
		return tagParts[0]
	}

	return ""
}

// copyMatchingFields copies matching fields from source to target
func copyMatchingFields(sourceVal, targetVal reflect.Value, targetFields map[string]reflect.Value) {
	_ = targetVal // Used for type checking in reflection operations
	sourceType := sourceVal.Type()

	for i := 0; i < sourceVal.NumField(); i++ {
		sourceField := sourceType.Field(i)
		if !sourceField.IsExported() {
			continue
		}

		sourceValue := sourceVal.Field(i)
		fieldName := getFieldMappingName(sourceField)

		targetField, found := targetFields[fieldName]
		if !found || !targetField.CanSet() {
			continue
		}

		tryConvertField(sourceValue, targetField)
	}
}

// getFieldMappingName gets the name to use for field mapping
func getFieldMappingName(field reflect.StructField) string {
	// Try JSON tag first
	if tag, ok := field.Tag.Lookup("json"); ok {
		tagName := extractJSONTagName(tag)
		if tagName != "" {
			return strings.ToLower(tagName)
		}
	}
	// Fall back to field name
	return strings.ToLower(field.Name)
}

// tryConvertField attempts to convert and assign a field value
func tryConvertField(sourceValue, targetField reflect.Value) {
	if canConvert(sourceValue, targetField) {
		convertValue(sourceValue, targetField)
	}
}

// canConvert checks if a source value can be converted to a target type
func canConvert(source reflect.Value, target reflect.Value) bool {
	if isNilValue(source) {
		return true
	}

	sourceType := source.Type()
	targetType := target.Type()

	return isDirectlyAssignable(sourceType, targetType) ||
		isConvertible(sourceType, targetType) ||
		areCompatiblePointers(source, target) ||
		areCompatibleCollections(sourceType, targetType)
}

// isNilValue checks if a value is nil (for pointers and interfaces)
func isNilValue(value reflect.Value) bool {
	return (value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface) && value.IsNil()
}

// isDirectlyAssignable checks if source type is directly assignable to target type
func isDirectlyAssignable(sourceType, targetType reflect.Type) bool {
	return sourceType.AssignableTo(targetType)
}

// isConvertible checks if source type is convertible to target type
func isConvertible(sourceType, targetType reflect.Type) bool {
	return sourceType.ConvertibleTo(targetType)
}

// areCompatiblePointers checks if pointer types are compatible
func areCompatiblePointers(source, target reflect.Value) bool {
	sourceType := source.Type()
	targetType := target.Type()

	if sourceType.Kind() == reflect.Ptr && targetType.Kind() == reflect.Ptr {
		return isCompatibleTypes(sourceType.Elem(), targetType.Elem())
	}

	if sourceType.Kind() == reflect.Ptr && targetType.Kind() != reflect.Ptr {
		return !source.IsNil() && isCompatibleTypes(sourceType.Elem(), targetType)
	}

	if sourceType.Kind() != reflect.Ptr && targetType.Kind() == reflect.Ptr {
		return isCompatibleTypes(sourceType, targetType.Elem())
	}

	return false
}

// areCompatibleCollections checks if slice or map types are compatible
func areCompatibleCollections(sourceType, targetType reflect.Type) bool {
	if sourceType.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice {
		return isCompatibleTypes(sourceType.Elem(), targetType.Elem())
	}

	if sourceType.Kind() == reflect.Map && targetType.Kind() == reflect.Map {
		return isCompatibleTypes(sourceType.Key(), targetType.Key()) &&
			isCompatibleTypes(sourceType.Elem(), targetType.Elem())
	}

	return false
}

// isCompatibleTypes checks if two types are compatible for conversion
func isCompatibleTypes(sourceType, targetType reflect.Type) bool {
	return sourceType.AssignableTo(targetType) || sourceType.ConvertibleTo(targetType)
}

// convertValue converts a source value to a target value
func convertValue(source reflect.Value, target reflect.Value) {
	if isNilValue(source) {
		handleNilValue(target)
		return
	}

	converter := getValueConverter(source, target)
	converter(source, target)
}

// ValueConverter represents a function that converts between two reflect.Value types
type ValueConverter func(source, target reflect.Value)

// getValueConverter returns the appropriate converter function
func getValueConverter(source, target reflect.Value) ValueConverter {
	sourceType := source.Type()
	targetType := target.Type()

	if isDirectlyAssignable(sourceType, targetType) {
		return directAssignment
	}

	if isConvertible(sourceType, targetType) {
		return typeConversion
	}

	if isTimeToTimeConversion(sourceType, targetType) {
		return timeConversion
	}

	if arePointerTypes(sourceType, targetType) {
		return pointerConversion
	}

	if isPointerToValueConversion(sourceType, targetType) {
		return pointerToValueConversion
	}

	if isValueToPointerConversion(sourceType, targetType) {
		return valueToPointerConversion
	}

	if areSliceTypes(sourceType, targetType) {
		return sliceConversion
	}

	if areMapTypes(sourceType, targetType) {
		return mapConversion
	}

	return noConversion
}

// handleNilValue handles nil source values
func handleNilValue(target reflect.Value) {
	if target.Kind() == reflect.Ptr || target.Kind() == reflect.Interface {
		target.Set(reflect.Zero(target.Type()))
	}
}

// Converter function implementations

func directAssignment(source, target reflect.Value) {
	target.Set(source)
}

func typeConversion(source, target reflect.Value) {
	target.Set(source.Convert(target.Type()))
}

func timeConversion(source, target reflect.Value) {
	target.Set(source)
}

func pointerConversion(source, target reflect.Value) {
	if target.IsNil() {
		target.Set(reflect.New(target.Type().Elem()))
	}

	if !source.IsNil() {
		convertPointedValues(source.Elem(), target.Elem())
	}
}

func pointerToValueConversion(source, target reflect.Value) {
	if !source.IsNil() {
		convertPointedValues(source.Elem(), target)
	}
}

func valueToPointerConversion(source, target reflect.Value) {
	newValue := reflect.New(target.Type().Elem())
	convertPointedValues(source, newValue.Elem())
	target.Set(newValue)
}

func sliceConversion(source, target reflect.Value) {
	sourceLen := source.Len()
	newSlice := reflect.MakeSlice(target.Type(), sourceLen, sourceLen)

	for i := 0; i < sourceLen; i++ {
		convertSliceElement(source.Index(i), newSlice.Index(i))
	}

	target.Set(newSlice)
}

func mapConversion(source, target reflect.Value) {
	sourceKeys := source.MapKeys()
	newMap := reflect.MakeMap(target.Type())

	for _, key := range sourceKeys {
		sourceVal := source.MapIndex(key)
		targetKey := convertMapKey(key, target.Type().Key())
		targetVal := convertMapValue(sourceVal, target.Type().Elem())
		newMap.SetMapIndex(targetKey, targetVal)
	}

	target.Set(newMap)
}

func noConversion(_, _ reflect.Value) {
	// No conversion possible, do nothing
}

// Helper functions for converters

func convertPointedValues(source, target reflect.Value) {
	if source.Type().AssignableTo(target.Type()) {
		target.Set(source)
	} else if source.Type().ConvertibleTo(target.Type()) {
		target.Set(source.Convert(target.Type()))
	}
}

func convertSliceElement(source, target reflect.Value) {
	if source.Type().AssignableTo(target.Type()) {
		target.Set(source)
	} else if source.Type().ConvertibleTo(target.Type()) {
		target.Set(source.Convert(target.Type()))
	}
}

func convertMapKey(key reflect.Value, targetType reflect.Type) reflect.Value {
	if key.Type().AssignableTo(targetType) {
		return key
	}

	return key.Convert(targetType)
}

func convertMapValue(value reflect.Value, targetType reflect.Type) reflect.Value {
	if value.Type().AssignableTo(targetType) {
		return value
	}

	return value.Convert(targetType)
}

// Type checking helper functions

func isTimeToTimeConversion(sourceType, targetType reflect.Type) bool {
	timeType := reflect.TypeOf(time.Time{})
	return sourceType == timeType && targetType == timeType
}

func arePointerTypes(sourceType, targetType reflect.Type) bool {
	return sourceType.Kind() == reflect.Ptr && targetType.Kind() == reflect.Ptr
}

func isPointerToValueConversion(sourceType, targetType reflect.Type) bool {
	return sourceType.Kind() == reflect.Ptr && targetType.Kind() != reflect.Ptr
}

func isValueToPointerConversion(sourceType, targetType reflect.Type) bool {
	return sourceType.Kind() != reflect.Ptr && targetType.Kind() == reflect.Ptr
}

func areSliceTypes(sourceType, targetType reflect.Type) bool {
	return sourceType.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice
}

func areMapTypes(sourceType, targetType reflect.Type) bool {
	return sourceType.Kind() == reflect.Map && targetType.Kind() == reflect.Map
}
