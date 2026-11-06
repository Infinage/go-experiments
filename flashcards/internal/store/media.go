package store

import "errors"

// Media is a card-front attachment with its content type.
type Media struct {
	Bytes []byte
	Mime  string
}

var ErrNoMedia = errors.New("card has no such media")

// SetImage stores image bytes (and their MIME type) on the card front.
func (s *Store) SetImage(cardID int64, m Media) error {
	return s.setMedia(cardID, m,
		`UPDATE cards SET front_image=?, front_image_mime=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`)
}

// SetAudio stores audio bytes (and their MIME type) on the card front.
func (s *Store) SetAudio(cardID int64, m Media) error {
	return s.setMedia(cardID, m,
		`UPDATE cards SET front_audio=?, front_audio_mime=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`)
}

func (s *Store) setMedia(cardID int64, m Media, query string) error {
	res, err := s.db.Exec(query, m.Bytes, m.Mime, cardID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	switch {
	case err != nil:
		return err
	case n == 0:
		return ErrNotFound
	}
	return nil
}

// Image returns the card-front image, or ErrNoMedia when absent.
func (s *Store) Image(cardID int64) (Media, error) {
	return s.media(cardID, "front_image", "front_image_mime")
}

// Audio returns the card-front audio, or ErrNoMedia when absent.
func (s *Store) Audio(cardID int64) (Media, error) {
	return s.media(cardID, "front_audio", "front_audio_mime")
}

func (s *Store) media(cardID int64, blobCol, mimeCol string) (Media, error) {
	var m Media
	var bytes, mime []byte
	err := s.db.QueryRow(
		`SELECT `+blobCol+`, `+mimeCol+` FROM cards WHERE id = ?`, cardID).
		Scan(&bytes, &mime)
	switch {
	case errors.Is(err, errNoRows):
		return m, ErrNotFound
	case err != nil:
		return m, err
	case bytes == nil:
		return m, ErrNoMedia
	}
	return Media{Bytes: bytes, Mime: string(mime)}, nil
}
