package ner

import (
	"strings"
	"testing"
)

func TestChunkText_ShortText(t *testing.T) {
	chunks := chunkText("hello world", 100)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].text != "hello world" || chunks[0].offset != 0 {
		t.Errorf("unexpected chunk: %+v", chunks[0])
	}
}

func TestChunkText_ExactLimit(t *testing.T) {
	text := strings.Repeat("a", 100)
	chunks := chunkText(text, 100)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestChunkText_SplitsOnNewline(t *testing.T) {
	text := "first paragraph\nsecond paragraph"
	chunks := chunkText(text, 20)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].text != "first paragraph\n" {
		t.Errorf("chunk 0: %q", chunks[0].text)
	}
	if chunks[0].offset != 0 {
		t.Errorf("chunk 0 offset: %d", chunks[0].offset)
	}
	if chunks[1].text != "second paragraph" {
		t.Errorf("chunk 1: %q", chunks[1].text)
	}
	if chunks[1].offset != 16 {
		t.Errorf("chunk 1 offset: %d, want 16", chunks[1].offset)
	}
}

func TestChunkText_SplitsOnSpace(t *testing.T) {
	// No newlines — should split on space
	text := "hello world foobar"
	chunks := chunkText(text, 14)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].text != "hello world " {
		t.Errorf("chunk 0: %q", chunks[0].text)
	}
	if chunks[1].text != "foobar" {
		t.Errorf("chunk 1: %q", chunks[1].text)
	}
	if chunks[1].offset != 12 {
		t.Errorf("chunk 1 offset: %d, want 12", chunks[1].offset)
	}
}

func TestChunkText_SplitsOnSpecialChars(t *testing.T) {
	// Dense HTML with no whitespace or newlines
	text := "<div>content</div><span>more</span>"
	chunks := chunkText(text, 20)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(chunks))
	}
	// Verify all chunks rejoin to original
	var combined string
	for _, c := range chunks {
		combined += c.text
	}
	if combined != text {
		t.Errorf("chunks don't rejoin:\n got: %q\nwant: %q", combined, text)
	}
}

func TestChunkText_HardCutFallback(t *testing.T) {
	// Single massive blob with zero break characters
	text := strings.Repeat("a", 30)
	chunks := chunkText(text, 10)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	var combined string
	for _, c := range chunks {
		combined += c.text
	}
	if combined != text {
		t.Errorf("chunks don't rejoin")
	}
}

func TestChunkText_OffsetsAreCorrect(t *testing.T) {
	text := "aaa bbb\nccc ddd\neee fff"
	chunks := chunkText(text, 10)
	for _, c := range chunks {
		actual := text[c.offset : c.offset+len(c.text)]
		if actual != c.text {
			t.Errorf("offset %d: text[offset:] = %q, chunk.text = %q", c.offset, actual, c.text)
		}
	}
}

func TestChunkText_Empty(t *testing.T) {
	chunks := chunkText("", 100)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty, got %d", len(chunks))
	}
	if chunks[0].text != "" {
		t.Errorf("expected empty text, got %q", chunks[0].text)
	}
}

func TestChunkText_MixedDelimiters(t *testing.T) {
	// Newlines preferred over spaces
	text := "word word\nword word\nword word"
	chunks := chunkText(text, 12)
	// Should split at first \n
	if chunks[0].text != "word word\n" {
		t.Errorf("expected newline split, got %q", chunks[0].text)
	}
}

func TestChunkText_TabSplit(t *testing.T) {
	text := "col1\tcol2\tcol3\tcol4"
	chunks := chunkText(text, 12)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(chunks))
	}
	var combined string
	for _, c := range chunks {
		combined += c.text
	}
	if combined != text {
		t.Errorf("chunks don't rejoin")
	}
}

func TestFindSplitPoint_PrefersNewline(t *testing.T) {
	window := "hello world\nfoo bar"
	idx := findSplitPoint(window)
	if idx != 12 { // right after \n
		t.Errorf("expected 12, got %d", idx)
	}
}

func TestFindSplitPoint_FallsBackToSpace(t *testing.T) {
	window := "hello world foo"
	idx := findSplitPoint(window)
	if idx != 12 { // right after last space
		t.Errorf("expected 12, got %d", idx)
	}
}

func TestFindSplitPoint_FallsBackToSpecial(t *testing.T) {
	window := "abcdef<ghijkl"
	idx := findSplitPoint(window)
	if idx != 7 { // right after <
		t.Errorf("expected 7, got %d", idx)
	}
}

func TestFindSplitPoint_HardCut(t *testing.T) {
	window := "abcdefghij"
	idx := findSplitPoint(window)
	if idx != 10 {
		t.Errorf("expected 10 (full window), got %d", idx)
	}
}

func TestChunkText_RealisticHTML(t *testing.T) {
	// Simulate pasted HTML like from the error report
	html := `<script>var checktimeout = 0;window.addEventListener("load", function () {` +
		`waitForElement(".cky-consent-container", function () {` +
		`const styleNode = document.getElementById("cky-style");` +
		`const clonedStyleNode = styleNode.cloneNode(true);` +
		`let lastUrl = location.href;` +
		`new MutationObserver(() => {const url = location.href;` +
		`if (url !== lastUrl) {lastUrl = url;onUrlChange();}` +
		`}).observe(document, { subtree: true, childList: true });` +
		`function onUrlChange() {document.head.appendChild(clonedStyleNode);}` +
		`});});</script>`

	chunks := chunkText(html, maxChunkChars)
	// Verify all chunks rejoin to original
	var combined string
	for _, c := range chunks {
		combined += c.text
	}
	if combined != html {
		t.Errorf("chunks don't rejoin to original")
	}
	// Verify offsets
	for _, c := range chunks {
		if text := html[c.offset : c.offset+len(c.text)]; text != c.text {
			t.Errorf("offset mismatch at %d", c.offset)
		}
	}
}

func TestChunkText_LongConversation(t *testing.T) {
	// Simulate a long conversation body with multiple paragraphs
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("This is paragraph number ")
		b.WriteString(strings.Repeat("word ", 20))
		b.WriteString("\n\n")
	}
	text := b.String()
	chunks := chunkText(text, maxChunkChars)

	// Verify integrity
	var combined string
	prevEnd := 0
	for _, c := range chunks {
		if c.offset != prevEnd {
			t.Errorf("gap at offset %d, expected %d", c.offset, prevEnd)
		}
		combined += c.text
		prevEnd = c.offset + len(c.text)
	}
	if combined != text {
		t.Errorf("chunks don't rejoin (len %d vs %d)", len(combined), len(text))
	}
}
