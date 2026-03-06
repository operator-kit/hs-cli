// Package ner provides ML-based named entity recognition for PII redaction.
// It uses ONNX Runtime (via purego) and a multilingual DistilBERT NER model
// to detect person names in freeform text.
//
// The model bundle is downloaded separately via `hs pii-model install` and cached
// in the user's OS-specific cache directory. When the bundle is not present,
// the NER detector cannot be created and freeform text is hidden instead.
package ner

import (
	"fmt"
	"strings"
	"sync"

	"github.com/operator-kit/hs-cli/internal/pii"
)

// maxSeqLen is the model's maximum input sequence length (position embeddings).
const maxSeqLen = 512

// maxChunkChars is a conservative character limit per chunk to stay under
// maxSeqLen tokens after tokenization. Average token ≈ 3-4 chars for
// multilingual text; 1200 chars ≈ 300-400 tokens, well within 512.
const maxChunkChars = 1200

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
// Long text is automatically chunked to stay within the model's 512-token limit.
func (d *Detector) DetectNames(text string) ([]pii.NameSpan, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	chunks := chunkText(text, maxChunkChars)
	var allSpans []pii.NameSpan

	for _, c := range chunks {
		spans, err := d.runChunk(c.text)
		if err != nil {
			continue // skip failed chunks, detect what we can
		}
		// Adjust offsets back to the original text.
		for i := range spans {
			spans[i].Start += c.offset
			spans[i].End += c.offset
		}
		allSpans = append(allSpans, spans...)
	}
	return allSpans, nil
}

// runChunk tokenizes and runs NER on a single chunk of text.
// If the tokenizer produces more than maxSeqLen tokens (unlikely given
// the conservative char limit, but possible with dense non-Latin text),
// the sequence is truncated as a safety net.
func (d *Detector) runChunk(text string) ([]pii.NameSpan, error) {
	enc, err := d.tokenizer.Encode(text)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	if len(enc.IDs) > maxSeqLen {
		enc.IDs = enc.IDs[:maxSeqLen]
		enc.AttentionMask = enc.AttentionMask[:maxSeqLen]
		if len(enc.Offsets) > maxSeqLen {
			enc.Offsets = enc.Offsets[:maxSeqLen]
		}
		if len(enc.Tokens) > maxSeqLen {
			enc.Tokens = enc.Tokens[:maxSeqLen]
		}
	}

	logits, err := d.runtime.Run(enc.IDs, enc.AttentionMask)
	if err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}

	tags := DecodeLogits(logits, d.labels, len(enc.IDs))
	return MergePersonSpans(tags, enc.Offsets, text), nil
}

// textChunk is a substring of the original text with its byte offset.
type textChunk struct {
	text   string
	offset int
}

// chunkText splits text into pieces of at most maxChars bytes, breaking at
// clean boundaries so we never split in the middle of a word or name.
//
// Split priority (highest to lowest):
//  1. Newline (\n)           — natural paragraph/line boundary
//  2. Whitespace (space/tab) — word boundary
//  3. Special character       — e.g. angle brackets, punctuation; handles
//     pasted HTML/code with no whitespace
//
// If none of the above are found (a single massive token with no breaks),
// we hard-cut at maxChars — this shouldn't affect name detection since such
// blobs are never natural language.
func chunkText(text string, maxChars int) []textChunk {
	if len(text) <= maxChars {
		return []textChunk{{text: text, offset: 0}}
	}

	var chunks []textChunk
	offset := 0

	for offset < len(text) {
		remaining := text[offset:]
		if len(remaining) <= maxChars {
			chunks = append(chunks, textChunk{text: remaining, offset: offset})
			break
		}

		window := remaining[:maxChars]
		split := findSplitPoint(window)
		chunk := remaining[:split]

		chunks = append(chunks, textChunk{text: chunk, offset: offset})
		offset += split
	}
	return chunks
}

// findSplitPoint returns the best byte index to cut a window of text.
// The returned index is always > 0 and includes the delimiter in the
// preceding chunk (so the next chunk starts cleanly).
func findSplitPoint(window string) int {
	// 1. Last newline
	if idx := strings.LastIndex(window, "\n"); idx > 0 {
		return idx + 1
	}
	// 2. Last whitespace (space or tab)
	if idx := strings.LastIndexAny(window, " \t"); idx > 0 {
		return idx + 1
	}
	// 3. Last special/punctuation character — catches dense HTML like
	//    "<div><span>...</span></div>" or "foo;bar;baz"
	if idx := strings.LastIndexAny(window, "<>{}()[];:,./\\|!@#$%^&*=+?\"'`~"); idx > 0 {
		return idx + 1
	}
	// 4. Hard cut — blob with zero break characters
	return len(window)
}

// Close releases ONNX Runtime resources.
func (d *Detector) Close() {
	if d.runtime != nil {
		d.runtime.Close()
	}
}
