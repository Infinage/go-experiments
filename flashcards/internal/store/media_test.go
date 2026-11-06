package store

import (
	"errors"
	"testing"
)

func TestMediaRoundTrip(t *testing.T) {
	s := mustStore(t)
	id, _ := s.CreateCard("q", "a", "")

	// Absent initially.
	if _, err := s.Image(id); !errors.Is(err, ErrNoMedia) {
		t.Fatalf("want ErrNoMedia, got %v", err)
	}
	if err := s.SetImage(id, Media{Bytes: []byte("png"), Mime: "image/png"}); err != nil {
		t.Fatal(err)
	}
	m, err := s.Image(id)
	if err != nil || string(m.Bytes) != "png" || m.Mime != "image/png" {
		t.Fatalf("image: %+v, %v", m, err)
	}
	// Flags reflect media presence.
	c, _ := s.Card(id)
	if !c.HasImage || c.HasAudio {
		t.Fatalf("flags: %+v", c)
	}

	// Audio is independent of image.
	if err := s.SetAudio(id, Media{Bytes: []byte("mp3"), Mime: "audio/mpeg"}); err != nil {
		t.Fatal(err)
	}
	m, _ = s.Audio(id)
	if string(m.Bytes) != "mp3" {
		t.Fatalf("audio: %+v", m)
	}
	// Replacing the image keeps the audio.
	if err := s.SetImage(id, Media{Bytes: []byte("png2"), Mime: "image/png"}); err != nil {
		t.Fatal(err)
	}
	if m, _ = s.Image(id); string(m.Bytes) != "png2" {
		t.Fatal("image not replaced")
	}

	// Deleting the card removes its media.
	if err := s.DeleteCard(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Image(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSetMediaMissingCard(t *testing.T) {
	s := mustStore(t)
	if err := s.SetImage(42, Media{Bytes: []byte("x"), Mime: "image/png"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestClearMedia(t *testing.T) {
	s := mustStore(t)
	id, _ := s.CreateCard("q", "a", "")
	s.SetImage(id, Media{Bytes: []byte("i"), Mime: "image/png"})
	s.SetAudio(id, Media{Bytes: []byte("a"), Mime: "audio/mpeg"})

	if err := s.ClearImage(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Image(id); !errors.Is(err, ErrNoMedia) {
		t.Fatalf("image should be gone: %v", err)
	}
	if _, err := s.Audio(id); err != nil {
		t.Fatalf("audio should survive: %v", err)
	}
	if err := s.ClearAudio(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Audio(id); !errors.Is(err, ErrNoMedia) {
		t.Fatalf("audio should be gone: %v", err)
	}
	if err := s.ClearImage(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing card: %v", err)
	}
}
