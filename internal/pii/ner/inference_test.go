package ner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLabels_Valid(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	data := `{"id2label":{"0":"O","1":"B-PER","2":"I-PER","3":"B-LOC","4":"I-LOC","5":"B-ORG","6":"I-ORG","7":"B-MISC","8":"I-MISC"}}`
	if err := os.WriteFile(config, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	labels, err := LoadLabels(config)
	if err != nil {
		t.Fatalf("LoadLabels: %v", err)
	}
	if len(labels) != 9 {
		t.Fatalf("expected 9 labels, got %d", len(labels))
	}
	if labels[0] != "O" {
		t.Fatalf("labels[0] = %q, want O", labels[0])
	}
	if labels[1] != "B-PER" {
		t.Fatalf("labels[1] = %q, want B-PER", labels[1])
	}
	if labels[8] != "I-MISC" {
		t.Fatalf("labels[8] = %q, want I-MISC", labels[8])
	}
}

func TestLoadLabels_MissingFile(t *testing.T) {
	_, err := LoadLabels("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadLabels_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	if err := os.WriteFile(config, []byte(`{invalid`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLabels(config)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadLabels_EmptyID2Label(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	if err := os.WriteFile(config, []byte(`{"id2label":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLabels(config)
	if err == nil {
		t.Fatal("expected error for empty id2label")
	}
}

func TestLoadLabels_NoID2LabelField(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	if err := os.WriteFile(config, []byte(`{"model_type":"bert"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLabels(config)
	if err == nil {
		t.Fatal("expected error when id2label missing")
	}
}

func TestLoadLabels_NonNumericKey(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	data := `{"id2label":{"zero":"O","one":"B-PER"}}`
	if err := os.WriteFile(config, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLabels(config)
	if err == nil {
		t.Fatal("expected error for non-numeric key")
	}
}

func TestLoadLabels_OutOfRangeID(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	// 2 entries but ID 5 is out of range
	data := `{"id2label":{"0":"O","5":"B-PER"}}`
	if err := os.WriteFile(config, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLabels(config)
	if err == nil {
		t.Fatal("expected error for out-of-range ID")
	}
}

func TestLoadLabels_WhitespaceTrimmed(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	data := `{"id2label":{"0":"  O  ","1":"  B-PER  "}}`
	if err := os.WriteFile(config, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	labels, err := LoadLabels(config)
	if err != nil {
		t.Fatalf("LoadLabels: %v", err)
	}
	if labels[0] != "O" {
		t.Fatalf("expected trimmed label, got %q", labels[0])
	}
}
