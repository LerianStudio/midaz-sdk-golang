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
	data, _ := json.Marshal(testData)

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
	json.NewEncoder(buf).Encode(testData)
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
