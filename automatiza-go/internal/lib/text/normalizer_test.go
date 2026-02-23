package text

import (
	"testing"
)

func TestNormalizeOutgoingWhatsAppText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes zero-width space U+200B",
			input: "hello\u200Bworld",
			want:  "helloworld",
		},
		{
			name:  "removes right-to-left mark U+200F",
			input: "hello\u200Fworld",
			want:  "helloworld",
		},
		{
			name:  "removes left-to-right embedding U+202A",
			input: "hello\u202Aworld",
			want:  "helloworld",
		},
		{
			name:  "removes word joiner U+2060",
			input: "hello\u2060world",
			want:  "helloworld",
		},
		{
			name:  "removes BOM U+FEFF",
			input: "\uFEFFhello world",
			want:  "hello world",
		},
		{
			name:  "removes multiple zero-width chars",
			input: "\u200B\u200F\u202A\u2060\uFEFFhello",
			want:  "hello",
		},
		{
			name:  "fixes CRLF to LF",
			input: "line1\r\nline2",
			want:  "line1\nline2",
		},
		{
			name:  "fixes CR to LF",
			input: "line1\rline2",
			want:  "line1\nline2",
		},
		{
			name:  "reduces 3+ newlines to 2",
			input: "line1\n\n\nline2",
			want:  "line1\n\nline2",
		},
		{
			name:  "reduces 5 newlines to 2",
			input: "line1\n\n\n\n\nline2",
			want:  "line1\n\nline2",
		},
		{
			name:  "trims whitespace from each line",
			input: "  hello  \n  world  ",
			want:  "hello\nworld",
		},
		{
			name:  "final trim",
			input: "  hello world  ",
			want:  "hello world",
		},
		{
			name:  "empty string returns empty",
			input: "",
			want:  "",
		},
		{
			name:  "already clean text unchanged",
			input: "hello world\nfoo bar",
			want:  "hello world\nfoo bar",
		},
		{
			name:  "all transformations combined",
			input: "  \u200Bhello\u200F  \r\n  world  \n\n\n\n  foo  ",
			want:  "hello\nworld\n\nfoo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeOutgoingWhatsAppText(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeOutgoingWhatsAppText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsEffectivelyEmptyWhatsAppText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  true,
		},
		{
			name:  "only whitespace",
			input: "   \t  \n  ",
			want:  true,
		},
		{
			name:  "only zero-width chars",
			input: "\u200B\u200F\u2060\uFEFF",
			want:  true,
		},
		{
			name:  "zero-width chars and whitespace",
			input: "  \u200B  \u200F  ",
			want:  true,
		},
		{
			name:  "has content",
			input: "hello",
			want:  false,
		},
		{
			name:  "has content with whitespace",
			input: "  hello  ",
			want:  false,
		},
		{
			name:  "has content with zero-width chars",
			input: "\u200Bhello\u200F",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsEffectivelyEmptyWhatsAppText(tt.input)
			if got != tt.want {
				t.Errorf("IsEffectivelyEmptyWhatsAppText(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanWhatsAppNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		phone string
		want  string
	}{
		{
			name:  "formatted Brazilian number with plus",
			phone: "+55 11 91234-5678",
			want:  "5511912345678",
		},
		{
			name:  "already clean number",
			phone: "5511912345678",
			want:  "5511912345678",
		},
		{
			name:  "parentheses format",
			phone: "(11) 91234-5678",
			want:  "11912345678",
		},
		{
			name:  "empty string",
			phone: "",
			want:  "",
		},
		{
			name:  "all non-numeric",
			phone: "abc-def",
			want:  "",
		},
		{
			name:  "mixed chars",
			phone: "+55 (11) 9.1234-5678",
			want:  "5511912345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CleanWhatsAppNumber(tt.phone)
			if got != tt.want {
				t.Errorf("CleanWhatsAppNumber(%q) = %q, want %q", tt.phone, got, tt.want)
			}
		})
	}
}

func TestFormatWhatsAppJID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		phone string
		want  string
	}{
		{
			name:  "clean number",
			phone: "5511912345678",
			want:  "5511912345678@s.whatsapp.net",
		},
		{
			name:  "number with special chars",
			phone: "+55 11 91234-5678",
			want:  "5511912345678@s.whatsapp.net",
		},
		{
			name:  "empty phone returns empty",
			phone: "",
			want:  "",
		},
		{
			name:  "all non-numeric returns empty",
			phone: "abc",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatWhatsAppJID(tt.phone)
			if got != tt.want {
				t.Errorf("FormatWhatsAppJID(%q) = %q, want %q", tt.phone, got, tt.want)
			}
		})
	}
}

func TestFormatWhatsAppGroupJID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groupID string
		want    string
	}{
		{
			name:    "already has suffix",
			groupID: "123456@g.us",
			want:    "123456@g.us",
		},
		{
			name:    "without suffix",
			groupID: "123456",
			want:    "123456@g.us",
		},
		{
			name:    "complex group ID without suffix",
			groupID: "120363123456789012",
			want:    "120363123456789012@g.us",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatWhatsAppGroupJID(tt.groupID)
			if got != tt.want {
				t.Errorf("FormatWhatsAppGroupJID(%q) = %q, want %q", tt.groupID, got, tt.want)
			}
		})
	}
}

func TestIsGroupJID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		jid  string
		want bool
	}{
		{
			name: "group JID",
			jid:  "123456@g.us",
			want: true,
		},
		{
			name: "user JID",
			jid:  "5511912345678@s.whatsapp.net",
			want: false,
		},
		{
			name: "empty string",
			jid:  "",
			want: false,
		},
		{
			name: "plain number",
			jid:  "123456",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsGroupJID(tt.jid)
			if got != tt.want {
				t.Errorf("IsGroupJID(%q) = %v, want %v", tt.jid, got, tt.want)
			}
		})
	}
}

func TestIsUserJID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		jid  string
		want bool
	}{
		{
			name: "user JID",
			jid:  "5511912345678@s.whatsapp.net",
			want: true,
		},
		{
			name: "group JID",
			jid:  "123456@g.us",
			want: false,
		},
		{
			name: "empty string",
			jid:  "",
			want: false,
		},
		{
			name: "plain number",
			jid:  "5511912345678",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsUserJID(tt.jid)
			if got != tt.want {
				t.Errorf("IsUserJID(%q) = %v, want %v", tt.jid, got, tt.want)
			}
		})
	}
}

func TestExtractPhoneFromJID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		jid  string
		want string
	}{
		{
			name: "user JID",
			jid:  "5511912345678@s.whatsapp.net",
			want: "5511912345678",
		},
		{
			name: "group JID",
			jid:  "123456@g.us",
			want: "123456",
		},
		{
			name: "no @ sign",
			jid:  "5511912345678",
			want: "5511912345678",
		},
		{
			name: "empty string",
			jid:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractPhoneFromJID(tt.jid)
			if got != tt.want {
				t.Errorf("ExtractPhoneFromJID(%q) = %q, want %q", tt.jid, got, tt.want)
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		maxLen int
		want   string
	}{
		{
			name:   "short text unchanged",
			text:   "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length unchanged",
			text:   "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "over limit truncated with ellipsis",
			text:   "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "maxLen equals 3 truncated without ellipsis",
			text:   "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "maxLen less than 3 truncated without ellipsis",
			text:   "hello",
			maxLen: 2,
			want:   "he",
		},
		{
			name:   "maxLen equals 1",
			text:   "hello",
			maxLen: 1,
			want:   "h",
		},
		{
			name:   "empty text returns empty",
			text:   "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "maxLen equals 4 with ellipsis",
			text:   "hello world",
			maxLen: 4,
			want:   "h...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TruncateText(tt.text, tt.maxLen)
			if got != tt.want {
				t.Errorf("TruncateText(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestRemoveEmojis(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "removes grinning face",
			text: "hello 😀 world",
			want: "hello  world",
		},
		{
			name: "removes house emoji",
			text: "home 🏠 sweet",
			want: "home  sweet",
		},
		{
			name: "removes scissors",
			text: "cut ✂ here",
			want: "cut  here",
		},
		{
			name: "keeps regular text",
			text: "hello world 123",
			want: "hello world 123",
		},
		{
			name: "removes multiple emojis",
			text: "😀🏠✂ text",
			want: " text",
		},
		{
			name: "empty string",
			text: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RemoveEmojis(tt.text)
			if got != tt.want {
				t.Errorf("RemoveEmojis(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestCollapseWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "multiple spaces to single",
			text: "hello    world",
			want: "hello world",
		},
		{
			name: "tabs to single space",
			text: "hello\t\tworld",
			want: "hello world",
		},
		{
			name: "spaces around newlines removed",
			text: "hello \nworld",
			want: "hello\nworld",
		},
		{
			name: "spaces before newline removed",
			text: "hello \n world",
			want: "hello\nworld",
		},
		{
			name: "trimmed",
			text: "  hello world  ",
			want: "hello world",
		},
		{
			name: "combined whitespace issues",
			text: "  hello    world  \n  foo   bar  ",
			want: "hello world\nfoo bar",
		},
		{
			name: "empty string",
			text: "",
			want: "",
		},
		{
			name: "only whitespace",
			text: "   \t  ",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CollapseWhitespace(tt.text)
			if got != tt.want {
				t.Errorf("CollapseWhitespace(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
