package glance

import "testing"

func TestSanitizeFeedDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{"plain text", "Hello world", "Hello world"},
		{"html tags", "<p>Hello <strong>world</strong></p>", "Hello world"},
		{"comment marker", "Intro text<!-- more -->", "Intro text"},
		{"comment without spaces", "<!--FULLCONTENT-->Intro text", "Intro text"},
		{"multiline comment", "Before <!-- line one\nline two --> after", "Before after"},
		{"two comments", "<!-- a -->One <!-- b -->two", "One two"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFeedDescription(tt.description); got != tt.want {
				t.Errorf("sanitizeFeedDescription(%q) = %q, want %q", tt.description, got, tt.want)
			}
		})
	}
}
