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
	sourceVal := reflect.ValueOf(source)
	targetVal := reflect.ValueOf(target)

	// Navigate pointers
	if sourceVal.Kind() == reflect.Ptr {
		if sourceVal.IsNil() {
			return nil // nothing to convert
		}
		sourceVal = sourceVal.Elem()
	}

	if targetVal.Kind() == reflect.Ptr {
		if targetVal.IsNil() {
			return nil // nowhere to store the result
		}
		targetVal = targetVal.Elem()
	}

	// Both source and target must be structs
	if sourceVal.Kind() != reflect.Struct || targetVal.Kind() != reflect.Struct {
		return nil
	}

	sourceType := sourceVal.Type()
	targetType := targetVal.Type()

	// Create a map of target field names for faster lookup
	targetFields := make(map[string]reflect.Value)
	for i := 0; i < targetVal.NumField(); i++ {
		field := targetType.Field(i)
		if field.IsExported() {
			// Use lowercase field name for case-insensitive matching
			targetFields[strings.ToLower(field.Name)] = targetVal.Field(i)

			// Also check JSON tag if present
			if tag, ok := field.Tag.Lookup("json"); ok {
				tagParts := strings.Split(tag, ",")
				if tagParts[0] != "" && tagParts[0] != "-" {
					targetFields[strings.ToLower(tagParts[0])] = targetVal.Field(i)
				}
			}
		}
	}

	// Copy matching fields from source to target
	for i := 0; i < sourceVal.NumField(); i++ {
		sourceField := sourceType.Field(i)
		if !sourceField.IsExported() {
			continue
		}

		// Try to match by field name
		fieldName := strings.ToLower(sourceField.Name)

		// Also check JSON tag if present
		if tag, ok := sourceField.Tag.Lookup("json"); ok {
			tagParts := strings.Split(tag, ",")
			if tagParts[0] != "" && tagParts[0] != "-" {
				fieldName = strings.ToLower(tagParts[0])
			}
		}

		// Find the target field
		targetField, found := targetFields[fieldName]
		if !found {
			continue // Skip if no matching field
		}

		// Skip if target field can't be set
		if !targetField.CanSet() {
			continue
		}

		sourceValue := sourceVal.Field(i)

		// Handle conversion between compatible types
		if canConvert(sourceValue, targetField) {
			convertValue(sourceValue, targetField)
		}
	}

	return nil
}

// canConvert checks if a source value can be converted to a target type
func canConvert(source reflect.Value, target reflect.Value) bool {
	// Handle nil source
	if (source.Kind() == reflect.Ptr || source.Kind() == reflect.Interface) && source.IsNil() {
		return true
	}

	sourceType := source.Type()
	targetType := target.Type()

	// Direct assignability
	if sourceType.AssignableTo(targetType) {
		return true
	}

	// Value conversion (int to int64, etc.)
	if sourceType.ConvertibleTo(targetType) {
		return true
	}

	// Handle pointers
	if sourceType.Kind() == reflect.Ptr && targetType.Kind() == reflect.Ptr {
		// Check if the pointed-to types are compatible
		return sourceType.Elem().AssignableTo(targetType.Elem()) ||
			sourceType.Elem().ConvertibleTo(targetType.Elem())
	}

	// Handle pointer to value or value to pointer conversions
	if sourceType.Kind() == reflect.Ptr && targetType.Kind() != reflect.Ptr {
		// Source is a pointer, target is a value
		if !source.IsNil() {
			elemSourceType := sourceType.Elem()
			return elemSourceType.AssignableTo(targetType) ||
				elemSourceType.ConvertibleTo(targetType)
		}
	}

	if sourceType.Kind() != reflect.Ptr && targetType.Kind() == reflect.Ptr {
		// Source is a value, target is a pointer
		elemTargetType := targetType.Elem()
		return sourceType.AssignableTo(elemTargetType) ||
			sourceType.ConvertibleTo(elemTargetType)
	}

	// Handle slices (if both are slices and element types are compatible)
	if sourceType.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice {
		return sourceType.Elem().AssignableTo(targetType.Elem()) ||
			sourceType.Elem().ConvertibleTo(targetType.Elem())
	}

	// Handle maps (if both are maps and key/value types are compatible)
	if sourceType.Kind() == reflect.Map && targetType.Kind() == reflect.Map {
		return (sourceType.Key().AssignableTo(targetType.Key()) ||
			sourceType.Key().ConvertibleTo(targetType.Key())) &&
			(sourceType.Elem().AssignableTo(targetType.Elem()) ||
				sourceType.Elem().ConvertibleTo(targetType.Elem()))
	}

	return false
}

// convertValue converts a source value to a target value
func convertValue(source reflect.Value, target reflect.Value) {
	// Handle nil source
	if (source.Kind() == reflect.Ptr || source.Kind() == reflect.Interface) && source.IsNil() {
		// If target is a pointer, set it to nil
		if target.Kind() == reflect.Ptr || target.Kind() == reflect.Interface {
			target.Set(reflect.Zero(target.Type()))
		}
		return
	}

	sourceType := source.Type()
	targetType := target.Type()

	// Direct assignment
	if sourceType.AssignableTo(targetType) {
		target.Set(source)
		return
	}

	// Value conversion
	if sourceType.ConvertibleTo(targetType) {
		target.Set(source.Convert(targetType))
		return
	}

	// Handle time.Time specifically
	if sourceType == reflect.TypeOf(time.Time{}) && targetType == reflect.TypeOf(time.Time{}) {
		target.Set(source)
		return
	}

	// Handle pointers
	if sourceType.Kind() == reflect.Ptr && targetType.Kind() == reflect.Ptr {
		// Create a new target value if needed
		if target.IsNil() {
			target.Set(reflect.New(targetType.Elem()))
		}

		// Convert the pointed-to values
		if !source.IsNil() {
			sourceElem := source.Elem()
			targetElem := target.Elem()

			if sourceElem.Type().AssignableTo(targetElem.Type()) {
				targetElem.Set(sourceElem)
			} else if sourceElem.Type().ConvertibleTo(targetElem.Type()) {
				targetElem.Set(sourceElem.Convert(targetElem.Type()))
			}
		}
		return
	}

	// Handle pointer to value conversion
	if sourceType.Kind() == reflect.Ptr && targetType.Kind() != reflect.Ptr {
		if !source.IsNil() {
			sourceElem := source.Elem()
			if sourceElem.Type().AssignableTo(targetType) {
				target.Set(sourceElem)
			} else if sourceElem.Type().ConvertibleTo(targetType) {
				target.Set(sourceElem.Convert(targetType))
			}
		}
		return
	}

	// Handle value to pointer conversion
	if sourceType.Kind() != reflect.Ptr && targetType.Kind() == reflect.Ptr {
		// Create a new target value
		newValue := reflect.New(targetType.Elem())

		// Set the target element
		if source.Type().AssignableTo(targetType.Elem()) {
			newValue.Elem().Set(source)
		} else if source.Type().ConvertibleTo(targetType.Elem()) {
			newValue.Elem().Set(source.Convert(targetType.Elem()))
		}

		target.Set(newValue)
		return
	}

	// Handle slices
	if sourceType.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice {
		sourceLen := source.Len()
		newSlice := reflect.MakeSlice(targetType, sourceLen, sourceLen)

		for i := 0; i < sourceLen; i++ {
			sourceItem := source.Index(i)
			targetItem := newSlice.Index(i)

			if sourceItem.Type().AssignableTo(targetItem.Type()) {
				targetItem.Set(sourceItem)
			} else if sourceItem.Type().ConvertibleTo(targetItem.Type()) {
				targetItem.Set(sourceItem.Convert(targetItem.Type()))
			}
		}

		target.Set(newSlice)
		return
	}

	// Handle maps
	if sourceType.Kind() == reflect.Map && targetType.Kind() == reflect.Map {
		sourceKeys := source.MapKeys()
		newMap := reflect.MakeMap(targetType)

		for _, key := range sourceKeys {
			sourceVal := source.MapIndex(key)
			var targetKey reflect.Value

			// Convert the key
			if key.Type().AssignableTo(targetType.Key()) {
				targetKey = key
			} else {
				targetKey = key.Convert(targetType.Key())
			}

			// Convert the value
			if sourceVal.Type().AssignableTo(targetType.Elem()) {
				newMap.SetMapIndex(targetKey, sourceVal)
			} else if sourceVal.Type().ConvertibleTo(targetType.Elem()) {
				newMap.SetMapIndex(targetKey, sourceVal.Convert(targetType.Elem()))
			}
		}

		target.Set(newMap)
		return
	}
}
