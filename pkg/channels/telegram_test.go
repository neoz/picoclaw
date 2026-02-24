package channels

import (
	"strings"
	"testing"
)

// --- markdownToTelegramHTML ---

func TestMarkdownToTelegramHTML_Empty(t *testing.T) {
	if got := markdownToTelegramHTML(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_PlainText(t *testing.T) {
	got := markdownToTelegramHTML("hello world")
	if got != "hello world" {
		t.Errorf("expected plain text unchanged, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_Bold(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"double asterisk", "**bold**", "<b>bold</b>"},
		{"double underscore", "__bold__", "<b>bold</b>"},
		{"mid sentence", "this is **bold** text", "this is <b>bold</b> text"},
		{"multiple", "**a** and **b**", "<b>a</b> and <b>b</b>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownToTelegramHTML(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMarkdownToTelegramHTML_Italic(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"simple", "_italic_", "<i>italic</i>"},
		{"mid sentence", "this is _italic_ text", "this is <i>italic</i> text"},
		{"multiple", "_a_ and _b_", "<i>a</i> and <i>b</i>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownToTelegramHTML(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMarkdownToTelegramHTML_Strikethrough(t *testing.T) {
	got := markdownToTelegramHTML("~~deleted~~")
	if got != "<s>deleted</s>" {
		t.Errorf("got %q", got)
	}
}

func TestMarkdownToTelegramHTML_Link(t *testing.T) {
	got := markdownToTelegramHTML("[click](https://example.com)")
	if got != `<a href="https://example.com">click</a>` {
		t.Errorf("got %q", got)
	}
}

func TestMarkdownToTelegramHTML_InlineCode(t *testing.T) {
	got := markdownToTelegramHTML("use `fmt.Println`")
	if got != "use <code>fmt.Println</code>" {
		t.Errorf("got %q", got)
	}
}

func TestMarkdownToTelegramHTML_CodeBlock(t *testing.T) {
	input := "before\n```go\nfmt.Println(\"hi\")\n```\nafter"
	got := markdownToTelegramHTML(input)
	if !strings.Contains(got, "<pre><code>") {
		t.Errorf("expected <pre><code>, got %q", got)
	}
	if !strings.Contains(got, "fmt.Println") {
		t.Errorf("expected code content preserved, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_CodeBlockHTMLEscaped(t *testing.T) {
	input := "```\n<div>&test</div>\n```"
	got := markdownToTelegramHTML(input)
	if !strings.Contains(got, "&lt;div&gt;") {
		t.Errorf("expected HTML escaped in code block, got %q", got)
	}
	if !strings.Contains(got, "&amp;test") {
		t.Errorf("expected & escaped in code block, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_InlineCodeHTMLEscaped(t *testing.T) {
	got := markdownToTelegramHTML("use `<b>tag</b>`")
	if !strings.Contains(got, "&lt;b&gt;") {
		t.Errorf("expected HTML escaped in inline code, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_HTMLEscaping(t *testing.T) {
	got := markdownToTelegramHTML("a < b & c > d")
	if got != "a &lt; b &amp; c &gt; d" {
		t.Errorf("got %q", got)
	}
}

func TestMarkdownToTelegramHTML_HeadingStripped(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"h1", "# Title", "Title"},
		{"h2", "## Section", "Section"},
		{"h3", "### Sub", "Sub"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownToTelegramHTML(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMarkdownToTelegramHTML_Blockquote(t *testing.T) {
	got := markdownToTelegramHTML("> quoted text")
	if got != "quoted text" {
		t.Errorf("got %q", got)
	}
}

func TestMarkdownToTelegramHTML_BoldAndItalicNoTagCrossing(t *testing.T) {
	// This was the bug: italic regex could wrap around bold HTML tags,
	// producing <i><b>text</i></b> (crossed tags).
	tests := []struct {
		name, input string
	}{
		{"bold inside italic attempt", "_**text_**"},
		{"italic around bold", "_text **bold** more_"},
		{"mixed markers", "**bold _text** italic_"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownToTelegramHTML(tt.input)
			assertNoTagCrossing(t, got)
		})
	}
}

func TestMarkdownToTelegramHTML_BoldItalicNested(t *testing.T) {
	// Properly nested: bold wrapping italic content
	got := markdownToTelegramHTML("**_nested_**")
	// Bold processes first: <b>_nested_</b>
	// Italic should match _nested_ inside: <b><i>nested</i></b>
	if got != "<b><i>nested</i></b>" {
		t.Errorf("got %q, want %q", got, "<b><i>nested</i></b>")
	}
}

func TestMarkdownToTelegramHTML_BoldLink(t *testing.T) {
	got := markdownToTelegramHTML("**[click](https://example.com)**")
	// Bold first: <b>[click](https://example.com)</b>
	// Link then: <b><a href="...">click</a></b>
	if got != `<b><a href="https://example.com">click</a></b>` {
		t.Errorf("got %q", got)
	}
}

func TestMarkdownToTelegramHTML_CodeNotFormatted(t *testing.T) {
	// Bold/italic markers inside code should not be formatted
	got := markdownToTelegramHTML("`**not bold**`")
	if strings.Contains(got, "<b>") {
		t.Errorf("bold should not apply inside inline code, got %q", got)
	}
	if !strings.Contains(got, "<code>") {
		t.Errorf("expected code tags, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_CodeBlockNotFormatted(t *testing.T) {
	input := "```\n**not bold** _not italic_\n```"
	got := markdownToTelegramHTML(input)
	if strings.Contains(got, "<b>") {
		t.Errorf("bold should not apply inside code block, got %q", got)
	}
	if strings.Contains(got, "<i>") {
		t.Errorf("italic should not apply inside code block, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_MultipleInlineCodes(t *testing.T) {
	got := markdownToTelegramHTML("`a` and `b`")
	if got != "<code>a</code> and <code>b</code>" {
		t.Errorf("got %q", got)
	}
}

// --- escapeHTML ---

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"no special", "no special"},
		{"<>&", "&lt;&gt;&amp;"},
		{"a<b>c&d", "a&lt;b&gt;c&amp;d"},
		{"", ""},
	}
	for _, tt := range tests {
		got := escapeHTML(tt.input)
		if got != tt.want {
			t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- extractCodeBlocks ---

func TestExtractCodeBlocks(t *testing.T) {
	input := "before\n```go\ncode here\n```\nafter"
	result := extractCodeBlocks(input)
	if len(result.codes) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(result.codes))
	}
	if !strings.Contains(result.codes[0], "code here") {
		t.Errorf("code block content = %q", result.codes[0])
	}
	if strings.Contains(result.text, "code here") {
		t.Error("placeholder text should not contain original code")
	}
	if !strings.Contains(result.text, "before") || !strings.Contains(result.text, "after") {
		t.Error("surrounding text should be preserved")
	}
}

func TestExtractCodeBlocks_Multiple(t *testing.T) {
	input := "```\nfirst\n```\nmid\n```\nsecond\n```"
	result := extractCodeBlocks(input)
	if len(result.codes) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(result.codes))
	}
}

func TestExtractCodeBlocks_None(t *testing.T) {
	result := extractCodeBlocks("no code blocks here")
	if len(result.codes) != 0 {
		t.Errorf("expected 0 code blocks, got %d", len(result.codes))
	}
	if result.text != "no code blocks here" {
		t.Errorf("text should be unchanged, got %q", result.text)
	}
}

// --- extractInlineCodes ---

func TestExtractInlineCodes(t *testing.T) {
	result := extractInlineCodes("use `foo` and `bar`")
	if len(result.codes) != 2 {
		t.Fatalf("expected 2 inline codes, got %d", len(result.codes))
	}
	if result.codes[0] != "foo" || result.codes[1] != "bar" {
		t.Errorf("codes = %v", result.codes)
	}
	if strings.Contains(result.text, "foo") || strings.Contains(result.text, "bar") {
		t.Error("placeholder text should not contain original code")
	}
}

func TestExtractInlineCodes_None(t *testing.T) {
	result := extractInlineCodes("no inline code")
	if len(result.codes) != 0 {
		t.Errorf("expected 0, got %d", len(result.codes))
	}
}

// --- splitMessage ---

func TestSplitMessage_Short(t *testing.T) {
	parts := splitMessage("short", 100)
	if len(parts) != 1 || parts[0] != "short" {
		t.Errorf("got %v", parts)
	}
}

func TestSplitMessage_ExactLimit(t *testing.T) {
	text := strings.Repeat("a", 100)
	parts := splitMessage(text, 100)
	if len(parts) != 1 {
		t.Errorf("expected 1 part, got %d", len(parts))
	}
}

func TestSplitMessage_SplitOnNewline(t *testing.T) {
	text := strings.Repeat("a", 40) + "\n" + strings.Repeat("b", 40)
	parts := splitMessage(text, 50)
	if len(parts) < 2 {
		t.Fatalf("expected >= 2 parts, got %d", len(parts))
	}
	// First part should end at the newline boundary
	if strings.Contains(parts[0], "b") {
		t.Errorf("first part should not contain 'b': %q", parts[0])
	}
}

func TestSplitMessage_SplitOnSpace(t *testing.T) {
	// No newlines, should split on space
	text := strings.Repeat("word ", 20) // 100 chars
	parts := splitMessage(text, 60)
	if len(parts) < 2 {
		t.Fatalf("expected >= 2 parts, got %d", len(parts))
	}
	for _, p := range parts {
		if len(p) > 60 {
			t.Errorf("part exceeds maxLen: len=%d", len(p))
		}
	}
}

func TestSplitMessage_PreservesAllContent(t *testing.T) {
	text := strings.Repeat("abcdefghij", 50) // 500 chars
	parts := splitMessage(text, 100)
	joined := strings.Join(parts, "")
	if joined != text {
		t.Errorf("joined parts differ from original: len(joined)=%d, len(original)=%d", len(joined), len(text))
	}
}

// --- parseChatID ---

func TestParseChatID(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"12345", 12345, false},
		{"-100123", -100123, false},
		{"0", 0, false},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := parseChatID(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseChatID(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("parseChatID(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- truncateString ---

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 3, "hel"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncateString(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

// --- extractEntityText ---

func TestExtractEntityText(t *testing.T) {
	// Basic ASCII
	got := extractEntityText("hello world", 6, 5)
	if got != "world" {
		t.Errorf("got %q, want %q", got, "world")
	}
}

func TestExtractEntityText_OutOfBounds(t *testing.T) {
	got := extractEntityText("short", 0, 100)
	if got != "" {
		t.Errorf("expected empty for out-of-bounds, got %q", got)
	}
}

// --- helpers ---

// assertNoTagCrossing checks that HTML tags in s are properly nested (no crossing).
func assertNoTagCrossing(t *testing.T, s string) {
	t.Helper()
	var stack []string
	i := 0
	for i < len(s) {
		if s[i] != '<' {
			i++
			continue
		}
		end := strings.IndexByte(s[i:], '>')
		if end == -1 {
			break
		}
		tag := s[i+1 : i+end]
		i += end + 1

		// Skip self-closing and attributes for href etc.
		if strings.HasPrefix(tag, "/") {
			closing := strings.TrimPrefix(tag, "/")
			if len(stack) == 0 {
				t.Errorf("unexpected closing tag </%s> with empty stack in %q", closing, s)
				return
			}
			top := stack[len(stack)-1]
			if top != closing {
				t.Errorf("tag crossing: expected </%s> but found </%s> in %q", top, closing, s)
				return
			}
			stack = stack[:len(stack)-1]
		} else {
			// Extract just tag name (strip attributes)
			name := strings.Fields(tag)[0]
			stack = append(stack, name)
		}
	}
}
