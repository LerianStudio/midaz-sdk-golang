package performance

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

// Test data structures
type testStruct struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Numbers []int        `json:"numbers"`
	Nested  nestedStruct `json:"nested"`
}

type nestedStruct struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

func generateTestData() testStruct {
	return testStruct{
		ID:      "test-123",
		Name:    "Test Object",
		Numbers: []int{1, 2, 3, 4, 5},
		Nested: nestedStruct{
			Field1: "Nested value",
			Field2: 42,
		},
	}
}

func TestJSONPoolMarshalUnmarshal(t *testing.T) {
	pool := NewJSONPool()
	testData := generateTestData()

	// Test Marshal
	data, err := pool.Marshal(testData)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify the marshaled data is correct by unmarshaling and comparing structs
	var unmarshaled testStruct
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if !reflect.DeepEqual(testData, unmarshaled) {
		t.Errorf("Marshaled data doesn't match expected.\nGot: %+v\nExpected: %+v", unmarshaled, testData)
	}

	// Test Unmarshal
	var result testStruct

	err = pool.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify the unmarshaled data matches the original
	if !reflect.DeepEqual(testData, result) {
		t.Errorf("Unmarshaled data doesn't match original.\nGot: %+v\nExpected: %+v", result, testData)
	}
}

func TestJSONPoolEncoderDecoder(t *testing.T) {
	pool := NewJSONPool()
	testData := generateTestData()

	// Test encoder
	buf := &bytes.Buffer{}
	enc := pool.NewEncoder(buf)

	err := enc.Encode(testData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	pool.ReleaseEncoder(enc)

	// Verify encoded data by decoding and comparing
	var decoded testStruct
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Failed to unmarshal encoded data: %v", err)
	}

	if !reflect.DeepEqual(testData, decoded) {
		t.Errorf("Encoded data doesn't match expected.\nGot: %+v\nExpected: %+v", decoded, testData)
	}

	// Test decoder
	var result testStruct

	dec := pool.NewDecoder(bytes.NewReader(buf.Bytes()))

	err = dec.Decode(&result)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	pool.ReleaseDecoder(dec)

	// Verify decoded data
	if !reflect.DeepEqual(testData, result) {
		t.Errorf("Decoded data doesn't match original.\nGot: %+v\nExpected: %+v", result, testData)
	}
}

func TestJSONPoolConcurrent(t *testing.T) {
	pool := NewJSONPool()
	testData := generateTestData()
	concurrency := 100
	iterations := 1000

	var wg sync.WaitGroup
	wg.Add(concurrency)

	errors := make(chan error, concurrency*iterations)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				// Marshal
				data, err := pool.Marshal(testData)
				if err != nil {
					errors <- err
					return
				}

				// Unmarshal
				var result testStruct

				err = pool.Unmarshal(data, &result)
				if err != nil {
					errors <- err
					return
				}

				// Verify result
				if !reflect.DeepEqual(testData, result) {
					errors <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Fatalf("Concurrent test failed: %v", err)
		}
	}
}

func BenchmarkMarshal(b *testing.B) {
	testData := generateTestData()

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Pool", func(b *testing.B) {
		pool := NewJSONPool()

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := pool.Marshal(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkUnmarshal(b *testing.B) {
	testData := generateTestData()

	data, err := json.Marshal(testData)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var result testStruct

			err := json.Unmarshal(data, &result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Pool", func(b *testing.B) {
		pool := NewJSONPool()

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var result testStruct

			err := pool.Unmarshal(data, &result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEncoder(b *testing.B) {
	testData := generateTestData()

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			enc := json.NewEncoder(buf)

			err := enc.Encode(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Pool", func(b *testing.B) {
		pool := NewJSONPool()

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			enc := pool.NewEncoder(buf)

			err := enc.Encode(testData)
			if err != nil {
				b.Fatal(err)
			}

			pool.ReleaseEncoder(enc)
		}
	})
}

func BenchmarkDecoder(b *testing.B) {
	testData := generateTestData()

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(testData); err != nil {
		b.Fatal(err)
	}

	data := buf.Bytes()

	b.Run("Standard", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var result testStruct

			dec := json.NewDecoder(bytes.NewReader(data))

			err := dec.Decode(&result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Pool", func(b *testing.B) {
		pool := NewJSONPool()

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var result testStruct

			dec := pool.NewDecoder(bytes.NewReader(data))

			err := dec.Decode(&result)
			if err != nil {
				b.Fatal(err)
			}

			pool.ReleaseDecoder(dec)
		}
	})
}

// Tests for DefaultJSONPool convenience functions

func TestDefaultJSONPoolFunctions(t *testing.T) {
	testData := generateTestData()

	t.Run("Marshal", func(t *testing.T) {
		data, err := Marshal(testData)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result testStruct
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if !reflect.DeepEqual(testData, result) {
			t.Error("Marshaled data doesn't match original")
		}
	})

	t.Run("Unmarshal", func(t *testing.T) {
		data, _ := json.Marshal(testData)

		var result testStruct

		err := Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if !reflect.DeepEqual(testData, result) {
			t.Error("Unmarshaled data doesn't match original")
		}
	})

	t.Run("NewEncoder", func(t *testing.T) {
		buf := &bytes.Buffer{}

		enc := NewEncoder(buf)
		if err := enc.Encode(testData); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		ReleaseEncoder(enc)

		var result testStruct
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if !reflect.DeepEqual(testData, result) {
			t.Error("Encoded data doesn't match original")
		}
	})

	t.Run("NewDecoder", func(t *testing.T) {
		data, _ := json.Marshal(testData)

		dec := NewDecoder(bytes.NewReader(data))

		var result testStruct
		if err := dec.Decode(&result); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		ReleaseDecoder(dec)

		if !reflect.DeepEqual(testData, result) {
			t.Error("Decoded data doesn't match original")
		}
	})
}

func TestJSONPool_MarshalError(t *testing.T) {
	pool := NewJSONPool()

	// Test marshaling a channel (which cannot be marshaled)
	ch := make(chan int)

	_, err := pool.Marshal(ch)
	if err == nil {
		t.Error("Expected error when marshaling channel, got nil")
	}
}

func TestJSONPool_UnmarshalError(t *testing.T) {
	pool := NewJSONPool()

	// Test unmarshaling invalid JSON
	invalidJSON := []byte(`{invalid json}`)

	var result testStruct

	err := pool.Unmarshal(invalidJSON, &result)
	if err == nil {
		t.Error("Expected error when unmarshaling invalid JSON, got nil")
	}
}

func TestJSONPool_LargeBuffer(t *testing.T) {
	pool := NewJSONPool()

	// Create a large data structure (>1MB)
	type largeStruct struct {
		Data []byte `json:"data"`
	}

	largeData := largeStruct{
		Data: make([]byte, 2*1024*1024), // 2MB
	}

	// Fill with some data
	for i := range largeData.Data {
		largeData.Data[i] = byte(i % 256)
	}

	// Marshal the large data
	data, err := pool.Marshal(largeData)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal and verify
	var result largeStruct

	err = pool.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(result.Data) != len(largeData.Data) {
		t.Errorf("Expected data length %d, got %d", len(largeData.Data), len(result.Data))
	}
}

func TestJSONPool_EmptyData(t *testing.T) {
	pool := NewJSONPool()

	t.Run("MarshalEmptyStruct", func(t *testing.T) {
		type emptyStruct struct{}

		data, err := pool.Marshal(emptyStruct{})
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		// Should produce "{}\n" (with newline from encoder)
		expected := "{}\n"
		if string(data) != expected {
			t.Errorf("Expected %q, got %q", expected, string(data))
		}
	})

	t.Run("MarshalNil", func(t *testing.T) {
		data, err := pool.Marshal(nil)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		// Should produce "null\n"
		expected := "null\n"
		if string(data) != expected {
			t.Errorf("Expected %q, got %q", expected, string(data))
		}
	})

	t.Run("UnmarshalEmptyJSON", func(t *testing.T) {
		emptyJSON := []byte(`{}`)

		var result testStruct

		err := pool.Unmarshal(emptyJSON, &result)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		// Result should be zero-value struct
		if result.ID != "" || result.Name != "" {
			t.Error("Expected zero-value struct")
		}
	})
}

func TestJSONPool_SpecialCharacters(t *testing.T) {
	pool := NewJSONPool()

	type specialCharsStruct struct {
		Text string `json:"text"`
	}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Unicode", "Hello, 世界!", "Hello, 世界!"},
		{"Emoji", "Hello 😀🎉", "Hello 😀🎉"},
		{"Quotes", `He said "Hello"`, `He said "Hello"`},
		{"Backslash", `Path: C:\Users\test`, `Path: C:\Users\test`},
		{"Newlines", "Line1\nLine2\nLine3", "Line1\nLine2\nLine3"},
		{"Tabs", "Col1\tCol2\tCol3", "Col1\tCol2\tCol3"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := specialCharsStruct{Text: tc.input}

			data, err := pool.Marshal(original)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var result specialCharsStruct

			err = pool.Unmarshal(data, &result)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if result.Text != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result.Text)
			}
		})
	}
}

func TestJSONPool_NestedStructures(t *testing.T) {
	pool := NewJSONPool()

	type deeplyNested struct {
		Level1 struct {
			Level2 struct {
				Level3 struct {
					Level4 struct {
						Value string `json:"value"`
					} `json:"level4"`
				} `json:"level3"`
			} `json:"level2"`
		} `json:"level1"`
	}

	original := deeplyNested{}
	original.Level1.Level2.Level3.Level4.Value = "deep value"

	data, err := pool.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result deeplyNested

	err = pool.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Level1.Level2.Level3.Level4.Value != "deep value" {
		t.Errorf("Expected 'deep value', got %q", result.Level1.Level2.Level3.Level4.Value)
	}
}

//nolint:revive // cognitive-complexity: comprehensive test with many sub-tests
func TestJSONPool_SlicesAndMaps(t *testing.T) {
	pool := NewJSONPool()

	t.Run("EmptySlice", func(t *testing.T) {
		var original []string

		data, err := pool.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result []string

		err = pool.Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result != nil {
			t.Error("Expected nil slice")
		}
	})

	t.Run("LargeSlice", func(t *testing.T) {
		original := make([]int, 10000)
		for i := range original {
			original[i] = i
		}

		data, err := pool.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result []int

		err = pool.Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if len(result) != len(original) {
			t.Errorf("Expected slice length %d, got %d", len(original), len(result))
		}
	})

	t.Run("ComplexMap", func(t *testing.T) {
		original := map[string]any{
			"string":  "value",
			"number":  42.5,
			"boolean": true,
			"null":    nil,
			"array":   []any{1, 2, 3},
			"object":  map[string]any{"nested": "value"},
		}

		data, err := pool.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result map[string]any

		err = pool.Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result["string"] != "value" {
			t.Errorf("Expected string='value', got %v", result["string"])
		}

		boolVal, ok := result["boolean"].(bool)
		if !ok || !boolVal {
			t.Errorf("Expected boolean=true, got %v", result["boolean"])
		}
	})
}

func TestJSONPool_BufferReuse(t *testing.T) {
	pool := NewJSONPool()

	// Perform multiple marshal operations to test buffer reuse
	for i := 0; i < 100; i++ {
		data := struct {
			Index int    `json:"index"`
			Value string `json:"value"`
		}{
			Index: i,
			Value: "test",
		}

		result, err := pool.Marshal(data)
		if err != nil {
			t.Fatalf("Marshal iteration %d failed: %v", i, err)
		}

		if len(result) == 0 {
			t.Errorf("Marshal iteration %d produced empty result", i)
		}
	}
}

func BenchmarkConcurrentMarshal(b *testing.B) {
	pool := NewJSONPool()
	testData := generateTestData()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := pool.Marshal(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkConcurrentUnmarshal(b *testing.B) {
	pool := NewJSONPool()
	testData := generateTestData()
	data, _ := pool.Marshal(testData)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var result testStruct

			err := pool.Unmarshal(data, &result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
