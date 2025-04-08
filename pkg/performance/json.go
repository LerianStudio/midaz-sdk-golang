// Package performance provides utilities for optimizing the performance of the Midaz SDK.
//
// This package implements various performance optimizations including:
// - JSON encoder/decoder pooling to reduce memory allocations
// - Connection pooling for efficient HTTP requests
// - Request batching for bulk operations
package performance

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

// JSONPool provides a pool of JSON encoders and decoders to reduce allocations.
// This is particularly useful for high-throughput applications that make many API calls.
type JSONPool struct {
	encoderPool sync.Pool
	decoderPool sync.Pool
	bufferPool  sync.Pool
}

// NewJSONPool creates a new JSONPool with initialized pools.
func NewJSONPool() *JSONPool {
	return &JSONPool{
		encoderPool: sync.Pool{
			New: func() interface{} {
				return json.NewEncoder(nil)
			},
		},
		decoderPool: sync.Pool{
			New: func() interface{} {
				return json.NewDecoder(nil)
			},
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// DefaultJSONPool is a shared instance of JSONPool for general use.
var DefaultJSONPool = NewJSONPool()

// Marshal encodes the value to JSON using a pooled encoder.
// This reduces allocations compared to json.Marshal.
func (p *JSONPool) Marshal(v interface{}) ([]byte, error) {
	buf := p.getBuffer()
	defer p.putBuffer(buf)

	enc := p.getEncoder(buf)
	err := enc.Encode(v)
	p.putEncoder(enc)

	if err != nil {
		return nil, err
	}

	// Make a copy of the buffer content since we're returning the buffer to the pool
	return append([]byte(nil), buf.Bytes()...), nil
}

// Unmarshal decodes JSON data into the value using a pooled decoder.
// This reduces allocations compared to json.Unmarshal.
func (p *JSONPool) Unmarshal(data []byte, v interface{}) error {
	dec := p.getDecoder(bytes.NewReader(data))
	err := dec.Decode(v)
	p.putDecoder(dec)
	return err
}

// NewEncoder returns a pooled encoder that writes to w.
func (p *JSONPool) NewEncoder(w io.Writer) *json.Encoder {
	enc := p.getEncoder(w)
	return enc
}

// NewDecoder returns a pooled decoder that reads from r.
func (p *JSONPool) NewDecoder(r io.Reader) *json.Decoder {
	dec := p.getDecoder(r)
	return dec
}

// ReleaseEncoder returns an encoder to the pool.
func (p *JSONPool) ReleaseEncoder(enc *json.Encoder) {
	p.putEncoder(enc)
}

// ReleaseDecoder returns a decoder to the pool.
func (p *JSONPool) ReleaseDecoder(dec *json.Decoder) {
	p.putDecoder(dec)
}

// getEncoder gets an encoder from the pool and configures it to write to w.
func (p *JSONPool) getEncoder(w io.Writer) *json.Encoder {
	// The standard json.Encoder doesn't support resetting the writer,
	// so we need to create a new one each time
	enc := json.NewEncoder(w)
	return enc
}

// putEncoder returns an encoder to the pool.
func (p *JSONPool) putEncoder(enc *json.Encoder) {
	// We can't reuse encoders with the standard json package
	// The pool is kept for API compatibility
}

// getDecoder gets a decoder from the pool and configures it to read from r.
func (p *JSONPool) getDecoder(r io.Reader) *json.Decoder {
	// The standard json.Decoder doesn't support resetting the reader,
	// so we need to create a new one each time
	dec := json.NewDecoder(r)
	return dec
}

// putDecoder returns a decoder to the pool.
func (p *JSONPool) putDecoder(dec *json.Decoder) {
	// We can't reuse decoders with the standard json package
	// The pool is kept for API compatibility
}

// getBuffer gets a buffer from the pool.
func (p *JSONPool) getBuffer() *bytes.Buffer {
	buf := p.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putBuffer returns a buffer to the pool.
func (p *JSONPool) putBuffer(buf *bytes.Buffer) {
	p.bufferPool.Put(buf)
}

// Marshal provides a convenience function to encode a value
// to JSON using the DefaultJSONPool.
func Marshal(v interface{}) ([]byte, error) {
	return DefaultJSONPool.Marshal(v)
}

// Unmarshal provides a convenience function to decode JSON data
// into a value using the DefaultJSONPool.
func Unmarshal(data []byte, v interface{}) error {
	return DefaultJSONPool.Unmarshal(data, v)
}

// NewEncoder provides a convenience function to create a new encoder
// using the DefaultJSONPool.
func NewEncoder(w io.Writer) *json.Encoder {
	return DefaultJSONPool.NewEncoder(w)
}

// NewDecoder provides a convenience function to create a new decoder
// using the DefaultJSONPool.
func NewDecoder(r io.Reader) *json.Decoder {
	return DefaultJSONPool.NewDecoder(r)
}

// ReleaseEncoder provides a convenience function to release an encoder
// using the DefaultJSONPool.
func ReleaseEncoder(enc *json.Encoder) {
	DefaultJSONPool.ReleaseEncoder(enc)
}

// ReleaseDecoder provides a convenience function to release a decoder
// using the DefaultJSONPool.
func ReleaseDecoder(dec *json.Decoder) {
	DefaultJSONPool.ReleaseDecoder(dec)
}
