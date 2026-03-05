//go:build !(darwin || linux || freebsd || netbsd)

package ner

import "fmt"

// Runtime wraps an ONNX Runtime session for NER inference.
// On this platform, ONNX Runtime via purego is not supported.
type Runtime struct{}

// NewRuntime is not supported on this platform.
func NewRuntime(*Paths) (*Runtime, error) {
	return nil, fmt.Errorf("onnx runtime not supported on this platform")
}

// Run is not supported on this platform.
func (r *Runtime) Run([]int64, []int64) ([][]float32, error) {
	return nil, fmt.Errorf("onnx runtime not supported on this platform")
}

// Close is a no-op on unsupported platforms.
func (r *Runtime) Close() {}
