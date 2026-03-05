// Package ner provides ML-based named entity recognition for PII redaction.
// It uses ONNX Runtime (via purego) and a multilingual DistilBERT NER model
// to detect person names in freeform text.
//
// The model bundle is downloaded separately via `hs ner install` and cached
// in the user's OS-specific cache directory. When the bundle is not present,
// the NER detector cannot be created and freeform text is hidden instead.
package ner

import (
	"fmt"
	"sync"

	"github.com/operator-kit/hs-cli/internal/pii"
)

// ModelVersion is the version tag used for bundle download URLs.
const ModelVersion = "1.0.0"

// Detector performs named entity recognition on text.
type Detector struct {
	mu        sync.Mutex
	runtime   *Runtime
	tokenizer *Tokenizer
	labels    []string
}

// NewDetector loads the ONNX Runtime and model from the cache directory.
// Returns an error if the model is not installed.
func NewDetector() (*Detector, error) {
	paths, err := ModelPaths()
	if err != nil {
		return nil, fmt.Errorf("ner model not ready: %w", err)
	}

	rt, err := NewRuntime(paths)
	if err != nil {
		return nil, fmt.Errorf("loading onnx runtime: %w", err)
	}

	tok, err := NewTokenizer(paths.TokenizerJSON)
	if err != nil {
		rt.Close()
		return nil, fmt.Errorf("loading tokenizer: %w", err)
	}

	labels, err := LoadLabels(paths.ConfigJSON)
	if err != nil {
		rt.Close()
		return nil, fmt.Errorf("loading label map: %w", err)
	}

	return &Detector{
		runtime:   rt,
		tokenizer: tok,
		labels:    labels,
	}, nil
}

// DetectNames returns person name spans found in text.
func (d *Detector) DetectNames(text string) ([]pii.NameSpan, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	enc, err := d.tokenizer.Encode(text)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	logits, err := d.runtime.Run(enc.IDs, enc.AttentionMask)
	if err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}

	tags := DecodeLogits(logits, d.labels, len(enc.IDs))
	spans := MergePersonSpans(tags, enc.Offsets, text)
	return spans, nil
}

// Close releases ONNX Runtime resources.
func (d *Detector) Close() {
	if d.runtime != nil {
		d.runtime.Close()
	}
}
