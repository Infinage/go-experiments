package store

import (
	"errors"
	"testing"
	"time"
)

func mustStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCardCRUD(t *testing.T) {
	s := mustStore(t)
	id, err := s.CreateCard("What is Go?", "A language", "langs")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Card(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.Front != "What is Go?" || c.Back != "A language" || c.Category != "langs" {
		t.Fatalf("got %+v", c)
	}
	if c.HasImage || c.HasAudio {
		t.Fatal("unexpected media flags")
	}
	if err := s.UpdateCard(id, "Go?", "A gopher", "tools"); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Card(id)
	if c.Front != "Go?" || c.Back != "A gopher" || c.Category != "tools" {
		t.Fatalf("update: got %+v", c)
	}
	if err := s.DeleteCard(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Card(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.DeleteCard(id); err != nil {
		t.Fatalf("delete of missing id should be a no-op, got %v", err)
	}
}

func TestCardsFilters(t *testing.T) {
	s := mustStore(t)
	for _, c := range [][]string{
		{"HTML meaning", "HyperText", "web"},
		{"CSS meaning", "Styles", "web"},
		{"TCP handshake", "SYN ACK", ""},
	} {
		if _, err := s.CreateCard(c[0], c[1], c[2]); err != nil {
			t.Fatal(err)
		}
	}
	// Give one card an image so the media filter has something to match.
	ids, _ := s.Cards("", "", "")
	if err := s.SetImage(ids[0].ID, Media{Bytes: []byte("i"), Mime: "image/png"}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		q, cat, media string
		want          int
	}{
		{"", "", "", 3},
		{"", "web", "", 2},
		{"meaning", "", "", 2},
		{"meaning", "web", "", 2},
		{"syn", "", "", 1},
		{"SYN", "", "", 1}, // LIKE is case-insensitive for ASCII
		{"zzz", "", "", 0},
		{"", "", "text", 2},
		{"", "", "image", 1},
		{"", "", "audio", 0},
	}
	for _, tc := range cases {
		got, err := s.Cards(tc.q, tc.cat, tc.media)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != tc.want {
			t.Errorf("Cards(%q,%q,%q) = %d cards, want %d",
				tc.q, tc.cat, tc.media, len(got), tc.want)
		}
	}
}

func TestCardsOrderNewestFirst(t *testing.T) {
	s := mustStore(t)
	for i := range 3 {
		s.CreateCard(string(rune('a'+i)), "", "")
	}
	cs, _ := s.Cards("", "", "")
	if cs[0].Front != "c" || cs[2].Front != "a" {
		t.Fatalf("order: %q %q %q", cs[0].Front, cs[1].Front, cs[2].Front)
	}
}

func TestBulkDelete(t *testing.T) {
	s := mustStore(t)
	var ids []int64
	for range 3 {
		id, _ := s.CreateCard("q", "a", "")
		ids = append(ids, id)
	}
	if err := s.DeleteCards(ids[:2]); err != nil {
		t.Fatal(err)
	}
	cs, _ := s.Cards("", "", "")
	if len(cs) != 1 {
		t.Fatalf("left %d cards", len(cs))
	}
}

func TestCategories(t *testing.T) {
	s := mustStore(t)
	s.CreateCard("a", "", "web")
	s.CreateCard("b", "", "web")
	s.CreateCard("c", "", "langs")
	s.CreateCard("d", "", "") // uncategorised, must be excluded
	cs, err := s.Categories()
	if err != nil {
		t.Fatal(err)
	}
	want := []Category{{"langs", 1}, {"web", 2}}
	if len(cs) != 2 || cs[0] != want[0] || cs[1] != want[1] {
		t.Fatalf("got %+v", cs)
	}
}

func TestCardStatus(t *testing.T) {
	s := mustStore(t)
	id, _ := s.CreateCard("q", "a", "")
	cs, _ := s.Cards("", "", "")
	if cs[0].Status != "new" {
		t.Fatalf("new card status: %q", cs[0].Status)
	}
	s.Review(id, 2, time.Now()) // Good -> learning
	cs, _ = s.Cards("", "", "")
	if cs[0].Status != "learning" {
		t.Fatalf("learning status: %q", cs[0].Status)
	}
	st, _ := s.Stats()
	if st.InProgress != 1 || st.NotStarted != 0 {
		t.Fatalf("stats: %+v", st)
	}
}
