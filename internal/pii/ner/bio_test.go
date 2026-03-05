package ner

import (
	"testing"
)

func TestMergePersonSpans_Basic(t *testing.T) {
	// Simulates: "[CLS] John Smith works here [SEP]"
	// Tokens:     [CLS]  John   Smith  works  here  [SEP]
	// Labels:       O    B-PER  I-PER    O      O      O
	tags := []tokenTag{
		{Label: "O", Confidence: 0.99},     // [CLS]
		{Label: "B-PER", Confidence: 0.95}, // John
		{Label: "I-PER", Confidence: 0.93}, // Smith
		{Label: "O", Confidence: 0.99},     // works
		{Label: "O", Confidence: 0.99},     // here
		{Label: "O", Confidence: 0.99},     // [SEP]
	}
	offsets := [][2]int{
		{0, 0},   // [CLS]
		{0, 4},   // John
		{5, 10},  // Smith
		{11, 16}, // works
		{17, 21}, // here
		{0, 0},   // [SEP]
	}
	text := "John Smith works here"

	spans := MergePersonSpans(tags, offsets, text)
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Text != "John Smith" {
		t.Fatalf("expected 'John Smith', got %q", spans[0].Text)
	}
	if spans[0].Start != 0 || spans[0].End != 10 {
		t.Fatalf("expected [0,10], got [%d,%d]", spans[0].Start, spans[0].End)
	}
}

func TestMergePersonSpans_MultipleNames(t *testing.T) {
	tags := []tokenTag{
		{Label: "O", Confidence: 0.99},     // [CLS]
		{Label: "B-PER", Confidence: 0.90}, // Alice
		{Label: "O", Confidence: 0.99},     // met
		{Label: "B-PER", Confidence: 0.88}, // Bob
		{Label: "I-PER", Confidence: 0.85}, // Jones
		{Label: "O", Confidence: 0.99},     // [SEP]
	}
	offsets := [][2]int{
		{0, 0},   // [CLS]
		{0, 5},   // Alice
		{6, 9},   // met
		{10, 13}, // Bob
		{14, 19}, // Jones
		{0, 0},   // [SEP]
	}
	text := "Alice met Bob Jones"

	spans := MergePersonSpans(tags, offsets, text)
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	if spans[0].Text != "Alice" {
		t.Fatalf("expected 'Alice', got %q", spans[0].Text)
	}
	if spans[1].Text != "Bob Jones" {
		t.Fatalf("expected 'Bob Jones', got %q", spans[1].Text)
	}
}

func TestMergePersonSpans_LowConfidence(t *testing.T) {
	tags := []tokenTag{
		{Label: "O", Confidence: 0.99},     // [CLS]
		{Label: "B-PER", Confidence: 0.40}, // Technical
		{Label: "I-PER", Confidence: 0.35}, // Support
		{Label: "O", Confidence: 0.99},     // [SEP]
	}
	offsets := [][2]int{
		{0, 0},   // [CLS]
		{0, 9},   // Technical
		{10, 17}, // Support
		{0, 0},   // [SEP]
	}
	text := "Technical Support"

	spans := MergePersonSpans(tags, offsets, text)
	if len(spans) != 0 {
		t.Fatalf("expected 0 spans (below confidence), got %d: %v", len(spans), spans)
	}
}

func TestMergePersonSpans_NoNames(t *testing.T) {
	tags := []tokenTag{
		{Label: "O", Confidence: 0.99},
		{Label: "O", Confidence: 0.99},
		{Label: "O", Confidence: 0.99},
	}
	offsets := [][2]int{
		{0, 0},
		{0, 5},
		{0, 0},
	}
	text := "hello"

	spans := MergePersonSpans(tags, offsets, text)
	if len(spans) != 0 {
		t.Fatalf("expected 0 spans, got %d", len(spans))
	}
}

func TestDecodeLogits(t *testing.T) {
	labels := []string{"O", "B-PER", "I-PER", "B-LOC", "I-LOC", "B-ORG", "I-ORG"}

	// Token 0: O wins (high logit at index 0)
	// Token 1: B-PER wins (high logit at index 1)
	logits := [][]float32{
		{10, -5, -5, -5, -5, -5, -5}, // O
		{-5, 10, -5, -5, -5, -5, -5}, // B-PER
	}

	tags := DecodeLogits(logits, labels, 2)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Label != "O" {
		t.Fatalf("expected O, got %q", tags[0].Label)
	}
	if tags[1].Label != "B-PER" {
		t.Fatalf("expected B-PER, got %q", tags[1].Label)
	}
	if tags[1].Confidence < 0.9 {
		t.Fatalf("expected high confidence for B-PER, got %f", tags[1].Confidence)
	}
}

func TestSoftmaxAt(t *testing.T) {
	logits := []float32{10, -5, -5}
	conf := softmaxAt(logits, 0)
	if conf < 0.99 {
		t.Fatalf("expected ~1.0, got %f", conf)
	}
}
