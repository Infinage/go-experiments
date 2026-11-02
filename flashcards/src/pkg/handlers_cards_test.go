package flashcards

import (
	"strings"
	"testing"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		sep  string
		want []string
	}{
		{"empty", "", ",", nil},
		{"single", "go", ",", []string{"go"}},
		{"trims and lowercases", " Go , Databases ", ",", []string{"go", "databases"}},
		{"dedupes", "go,GO,go", ",", []string{"go"}},
		{"custom separator", "a;b;c", ";", []string{"a", "b", "c"}},
		{"blank items dropped", ",a,,b,", ",", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTags(tt.raw, tt.sep)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("parseTags(%q, %q) = %v, want %v", tt.raw, tt.sep, got, tt.want)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name      string
		csv       string
		wantCards int
		wantErrs  int
	}{
		{"valid with tags", "prompt,answer,tags\nQ1,A1,go;db\n", 1, 0},
		{"valid without tags", "prompt,answer\nQ1,A1\nQ2,A2\n", 2, 0},
		{"BOM in header", "\ufeffprompt,answer\nQ1,A1\n", 1, 0},
		{"missing prompt column", "question,answer\nQ,A\n", 0, 1},
		{"missing answer column", "prompt\nQ\n", 0, 1},
		// A short row yields an empty answer field, so it produces an error row
		// while earlier valid rows are still kept.
		{"missing field in row", "prompt,answer\nQ1,A1\nQ2\n", 1, 1},
		// Header-only files parse cleanly; the caller reports the empty import.
		{"empty data rows only", "prompt,answer\n", 0, 0},
		{"blank required field", "prompt,answer\nQ1,\n", 0, 1},
		{"extra columns ignored", "extra,prompt,answer\nx,Q1,A1\n", 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards, errs := parseCSV(strings.NewReader(tt.csv), "alice")
			if len(cards) != tt.wantCards {
				t.Errorf("got %d cards, want %d (errs: %v)", len(cards), tt.wantCards, errs)
			}
			if len(errs) != tt.wantErrs {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

func TestParseCSVParsesTags(t *testing.T) {
	cards, errs := parseCSV(strings.NewReader("prompt,answer,tags\nQ1,A1,Go; DB ;go\n"), "alice")
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := []string{"go", "db"}
	if len(cards[0].Tags) != len(want) {
		t.Fatalf("tags = %v, want %v", cards[0].Tags, want)
	}
	for i, w := range want {
		if cards[0].Tags[i] != w {
			t.Errorf("tags[%d] = %q, want %q", i, cards[0].Tags[i], w)
		}
	}
	if cards[0].Username != "alice" {
		t.Errorf("username = %q, want %q", cards[0].Username, "alice")
	}
}
