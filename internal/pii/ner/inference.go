package ner

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
)

// Encoding holds tokenizer output.
type Encoding struct {
	IDs           []int64
	AttentionMask []int64
	Offsets       [][2]int // (start, end) byte offsets into original text per token
	Tokens        []string
}

// Tokenizer wraps a HuggingFace tokenizer loaded from tokenizer.json.
type Tokenizer struct {
	inner *tokenizer.Tokenizer
}

// NewTokenizer loads a tokenizer from a tokenizer.json file.
func NewTokenizer(path string) (*Tokenizer, error) {
	tk, err := pretrained.FromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}
	return &Tokenizer{inner: tk}, nil
}

// Encode tokenizes text and returns IDs, attention mask, and offsets.
func (t *Tokenizer) Encode(text string) (*Encoding, error) {
	enc, err := t.inner.EncodeSingle(text, true) // addSpecialTokens=true
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	n := len(enc.Ids)
	ids := make([]int64, n)
	mask := make([]int64, n)
	offsets := make([][2]int, n)

	for i := 0; i < n; i++ {
		ids[i] = int64(enc.Ids[i])
		mask[i] = int64(enc.AttentionMask[i])
		if i < len(enc.Offsets) && len(enc.Offsets[i]) >= 2 {
			offsets[i] = [2]int{enc.Offsets[i][0], enc.Offsets[i][1]}
		}
	}

	tokens := make([]string, n)
	copy(tokens, enc.Tokens)

	return &Encoding{
		IDs:           ids,
		AttentionMask: mask,
		Offsets:       offsets,
		Tokens:        tokens,
	}, nil
}

// labelConfig represents the id2label mapping from config.json.
type labelConfig struct {
	ID2Label map[string]string `json:"id2label"`
}

// LoadLabels reads the id2label mapping from config.json and returns
// an ordered slice where index = label ID.
func LoadLabels(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg labelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.ID2Label) == 0 {
		return nil, fmt.Errorf("no id2label in config")
	}

	labels := make([]string, len(cfg.ID2Label))
	for idStr, label := range cfg.ID2Label {
		var id int
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			return nil, fmt.Errorf("invalid label id %q: %w", idStr, err)
		}
		if id < 0 || id >= len(labels) {
			return nil, fmt.Errorf("label id %d out of range", id)
		}
		labels[id] = strings.TrimSpace(label)
	}
	return labels, nil
}
