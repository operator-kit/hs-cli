package ner

import (
	"math"
	"strings"

	"github.com/operator-kit/hs-cli/internal/pii"
)

// MinConfidence is the default threshold for accepting a name span.
const MinConfidence float32 = 0.7

// tokenTag holds the BIO label and confidence for one token.
type tokenTag struct {
	Label      string
	Confidence float32
	Start      int
	End        int
}

// DecodeLogits converts raw logits to BIO tags via argmax.
func DecodeLogits(logits [][]float32, labels []string, seqLen int) []tokenTag {
	tags := make([]tokenTag, seqLen)
	for i := 0; i < seqLen; i++ {
		if i >= len(logits) {
			tags[i] = tokenTag{Label: "O"}
			continue
		}
		row := logits[i]
		bestIdx := 0
		bestVal := float32(math.Inf(-1))
		for j, v := range row {
			if v > bestVal {
				bestVal = v
				bestIdx = j
			}
		}
		// Softmax for confidence
		conf := softmaxAt(row, bestIdx)
		label := "O"
		if bestIdx < len(labels) {
			label = labels[bestIdx]
		}
		tags[i] = tokenTag{Label: label, Confidence: conf}
	}
	return tags
}

// MergePersonSpans groups B-PER + I-PER sequences into NameSpans using offsets.
// Special tokens (offset 0,0) and non-PER labels are skipped.
func MergePersonSpans(tags []tokenTag, offsets [][2]int, text string) []pii.NameSpan {
	var spans []pii.NameSpan
	var current *pii.NameSpan
	var scores []float32

	flush := func() {
		if current == nil {
			return
		}
		avg := float32(0)
		for _, s := range scores {
			avg += s
		}
		avg /= float32(len(scores))
		current.Score = avg

		if avg >= MinConfidence {
			current.Text = strings.TrimSpace(text[current.Start:current.End])
			if current.Text != "" {
				spans = append(spans, *current)
			}
		}
		current = nil
		scores = nil
	}

	for i, tag := range tags {
		if i >= len(offsets) {
			break
		}
		off := offsets[i]
		// Skip special tokens ([CLS], [SEP], [PAD])
		if off[0] == 0 && off[1] == 0 {
			flush()
			continue
		}

		switch {
		case tag.Label == "B-PER":
			flush()
			current = &pii.NameSpan{Start: off[0], End: off[1]}
			scores = []float32{tag.Confidence}

		case tag.Label == "I-PER" && current != nil:
			current.End = off[1]
			scores = append(scores, tag.Confidence)

		default:
			flush()
		}
	}
	flush()
	return spans
}

func softmaxAt(logits []float32, idx int) float32 {
	max := float32(math.Inf(-1))
	for _, v := range logits {
		if v > max {
			max = v
		}
	}
	sumExp := float32(0)
	for _, v := range logits {
		sumExp += float32(math.Exp(float64(v - max)))
	}
	return float32(math.Exp(float64(logits[idx]-max))) / sumExp
}
