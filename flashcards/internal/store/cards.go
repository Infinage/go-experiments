package store

import (
	"database/sql"
	"errors"
	"strings"
)

var ErrNotFound = errors.New("not found")

type Card struct {
	ID    int64
	Front string
	Back  string
	// Category is "" when uncategorised.
	Category string

	HasImage bool
	HasAudio bool
	CreatedAt,
	UpdatedAt string

	// Status is the review progress: new, learning or mastered.
	Status string
}

// Category is a distinct card category with its card count.
type Category struct {
	Name  string
	Count int
}

// NoneFilter is the sentinel category value that selects cards
// without a category ("tags IS EMPTY"). Reserved by the save rule, so
// it can never collide with a real category name.
const NoneFilter = "__none__"

func (c *Card) scanFrom(row interface{ Scan(...any) error }) error {
	// HasImage/HasAudio come from "col IS NOT NULL" expressions, which
	// scan directly into bools (never NULL themselves).
	return row.Scan(&c.ID, &c.Front, &c.Back, &c.Category,
		&c.HasImage, &c.HasAudio, &c.CreatedAt, &c.UpdatedAt, &c.Status)
}

const cardCols = `id, front_text, back_text,
	IFNULL(category,''), front_image IS NOT NULL, front_audio IS NOT NULL, created_at, updated_at`

// cardStatusExpr maps a review interval to a status label.
const cardStatusExpr = `CASE
	WHEN rs.card_id IS NULL THEN 'new'
	WHEN IFNULL(rs.interval_days, 0) >= 21 THEN 'mastered'
	ELSE 'learning' END`

func (s *Store) CreateCard(front, back, category string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO cards(front_text, back_text, category) VALUES(?,?,?)`,
		front, back, category)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) Card(id int64) (Card, error) {
	var c Card
	err := s.db.QueryRow(`SELECT `+cardCols+` FROM cards WHERE id = ?`, id).
		Scan(&c.ID, &c.Front, &c.Back, &c.Category, &c.HasImage, &c.HasAudio, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// Cards lists cards newest-first, optionally filtered by category, media
// type ("any", "text", "image", "audio") and a substring match on
// front/back text. Empty values mean no filter. Each card carries its
// review status.
func (s *Store) Cards(q, category, media string) ([]Card, error) {
	where, args := []string{"1=1"}, []any{}
	catExpr, catArg := "category = ?", category
	if category == NoneFilter {
		catExpr, catArg = "IFNULL(category,'') = ''", ""
	}
	if q != "" {
		// Match front/back text and category ("tag") names.
		where = append(where,
			"(front_text LIKE ? OR back_text LIKE ? OR category LIKE ?)")
		pat := "%" + q + "%"
		args = append(args, pat, pat, pat)
	}
	if category != "" {
		where = append(where, catExpr)
		args = append(args, catArg)
	}
	switch media {
	case "text":
		where = append(where,
			"front_image IS NULL AND front_audio IS NULL")
	case "image":
		where = append(where, "front_image IS NOT NULL")
	case "audio":
		where = append(where, "front_audio IS NOT NULL")
	}
	rows, err := s.db.Query(`SELECT `+cardCols+`, `+cardStatusExpr+`
		FROM cards
		LEFT JOIN review_state rs ON rs.card_id = cards.id
		WHERE `+strings.Join(where, " AND ")+` ORDER BY id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		var c Card
		if err := c.scanFrom(rows); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CardsByIDs returns the given cards with their study state, in the
// database's id order (callers reorder as needed). Unknown ids are skipped.
func (s *Store) CardsByIDs(ids []int64) ([]StudyCard, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.Query(`SELECT `+cardCols+`, `+cardStatusExpr+`,
		IFNULL(rs.due_at, ''), IFNULL(rs.interval_days, 0),
		IFNULL(rs.ease, 2.0), IFNULL(rs.repetitions, 0),
		IFNULL(rs.lapses, 0), rs.card_id IS NULL
		FROM cards
		LEFT JOIN review_state rs ON rs.card_id = cards.id
		WHERE cards.id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StudyCard
	for rows.Next() {
		var sc StudyCard
		var due string
		if err := rows.Scan(&sc.ID, &sc.Front, &sc.Back, &sc.Category,
			&sc.HasImage, &sc.HasAudio, &sc.CreatedAt, &sc.UpdatedAt,
			&sc.Status, &due, &sc.State.Interval, &sc.State.Ease,
			&sc.State.Repetitions, &sc.State.Lapses, &sc.New); err != nil {
			return nil, err
		}
		sc.State.DueAt = parseTime(due)
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) UpdateCard(id int64, front, back, category string) error {
	res, err := s.db.Exec(
		`UPDATE cards SET front_text=?, back_text=?, category=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`,
		front, back, category, id)
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

func (s *Store) DeleteCard(id int64) error { return s.deleteCards([]int64{id}) }

// DeleteCards removes all cards with the given ids.
func (s *Store) DeleteCards(ids []int64) error { return s.deleteCards(ids) }

func (s *Store) deleteCards(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := s.db.Exec(`DELETE FROM cards WHERE id IN (`+ph+`)`, args...)
	return err
}

// Categories returns distinct categories with card counts, alphabetically.
// The "" category (uncategorised) is excluded.
// Categories returns distinct categories with card counts, alphabetically.
// The "" category (uncategorised) is excluded.
// UntaggedCount returns the number of cards without a category.
func (s *Store) UntaggedCount() (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM cards WHERE IFNULL(category,'') = ''`).Scan(&n)
	return n, err
}

// CategoryPending counts the cards still pending review per category,
// grouped by IFNULL(category,”) — the "" group is the untagged cards.
// Pending mirrors the study queue predicate: never reviewed, or due
// right now (mastered cards count once they come due again).
func (s *Store) CategoryPending() ([]Category, error) {
	rows, err := s.db.Query(`SELECT IFNULL(category,''), COUNT(*)
		FROM cards c
		LEFT JOIN review_state rs ON rs.card_id = c.id
		WHERE rs.card_id IS NULL
			OR rs.due_at <= strftime('%Y-%m-%dT%H:%M:%fZ','now')
		GROUP BY IFNULL(category,'') ORDER BY IFNULL(category,'')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Name, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Categories() ([]Category, error) {
	rows, err := s.db.Query(
		`SELECT category, COUNT(*) FROM cards WHERE IFNULL(category,'') <> '' GROUP BY category ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Name, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
