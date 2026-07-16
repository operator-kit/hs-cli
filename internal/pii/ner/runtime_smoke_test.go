package ner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/operator-kit/hs-cli/internal/pii"
)

type runtimePrivacyCorpus struct {
	Schema int                        `json:"schema"`
	Cases  []runtimePrivacyCorpusCase `json:"cases"`
}

type runtimePrivacyCorpusCase struct {
	ID           string   `json:"id"`
	Text         string   `json:"text"`
	RepeatPrefix string   `json:"repeat_prefix"`
	Repeat       int      `json:"repeat"`
	DetectNames  []string `json:"detect_names"`
}

func TestRuntimeBundleSmoke(t *testing.T) {
	dir := os.Getenv("HS_PII_MODEL_SMOKE_DIR")
	if dir == "" {
		t.Skip("set HS_PII_MODEL_SMOKE_DIR to execute the real runtime/model smoke test")
	}
	platform := CurrentPlatform()
	if capability := RuntimeCapabilityFor(platform); !capability.Supported {
		t.Fatalf("workflow attempted smoke test on unsupported target %s", platform.Key())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := validateRuntimeBundle(ctx, pathsAt(dir, platform)); err != nil {
		t.Fatalf("runtime/model smoke validation: %v", err)
	}

	t.Run("multilingual privacy corpus", func(t *testing.T) {
		evaluateRuntimePrivacyCorpus(t, pathsAt(dir, platform))
	})
}

func evaluateRuntimePrivacyCorpus(t *testing.T, paths *Paths) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "multilingual_privacy_corpus.json"))
	if err != nil {
		t.Fatalf("read privacy corpus: %v", err)
	}
	var corpus runtimePrivacyCorpus
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode privacy corpus: %v", err)
	}
	if corpus.Schema != 1 || len(corpus.Cases) == 0 {
		t.Fatalf("unsupported or empty privacy corpus")
	}

	detector, err := newDetectorFromPaths(paths)
	if err != nil {
		t.Fatalf("load detector for privacy corpus: %v", err)
	}
	defer detector.Close()

	started := time.Now()
	expectedNames := 0
	coveredNames := 0
	unexpectedNames := 0
	for _, fixture := range corpus.Cases {
		text := strings.Repeat(fixture.RepeatPrefix, fixture.Repeat) + fixture.Text
		spans, detectErr := detector.DetectNames(text)
		if detectErr != nil {
			t.Errorf("%s: detect names: %v", fixture.ID, detectErr)
			continue
		}

		expectedRanges := make([][2]int, 0, len(fixture.DetectNames))
		for _, expected := range fixture.DetectNames {
			start := strings.Index(text, expected)
			if start < 0 {
				t.Errorf("%s: expected name %q is absent from fixture", fixture.ID, expected)
				continue
			}
			rangeEnd := start + len(expected)
			expectedRanges = append(expectedRanges, [2]int{start, rangeEnd})
			expectedNames++
			if spanCoversRange(spans, start, rangeEnd) {
				coveredNames++
			} else {
				t.Errorf("%s: detector did not cover complete expected name %q; spans=%v", fixture.ID, expected, spans)
			}
		}

		for _, span := range spans {
			if !spanOverlapsAnyRange(span.Start, span.End, expectedRanges) {
				unexpectedNames++
				t.Errorf("%s: unexpected person span %q at [%d:%d]", fixture.ID, span.Text, span.Start, span.End)
			}
		}
	}

	elapsed := time.Since(started)
	t.Logf("privacy corpus: covered=%d/%d unexpected=%d elapsed=%s", coveredNames, expectedNames, unexpectedNames, elapsed.Round(time.Millisecond))
	if expectedNames == 0 || coveredNames != expectedNames {
		t.Errorf("privacy corpus full-name recall = %d/%d", coveredNames, expectedNames)
	}
	if unexpectedNames != 0 {
		t.Errorf("privacy corpus unexpected person spans = %d", unexpectedNames)
	}
	if elapsed > 2*time.Minute {
		t.Errorf("privacy corpus exceeded two-minute runtime budget: %s", elapsed)
	}
}

func spanCoversRange(spans []pii.NameSpan, start, end int) bool {
	for _, span := range spans {
		if span.Start <= start && span.End >= end {
			return true
		}
	}
	return false
}

func spanOverlapsAnyRange(start, end int, ranges [][2]int) bool {
	for _, expected := range ranges {
		if start < expected[1] && end > expected[0] {
			return true
		}
	}
	return false
}
