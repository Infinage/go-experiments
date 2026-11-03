package flashcards

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Init creates necessary database tables for users, cards, tags, and stats.
func (s *Store) Init(ctx context.Context) error {
	initUsers := `
		CREATE TABLE IF NOT EXISTS users (
			username TEXT PRIMARY KEY NOT NULL,
			password TEXT NOT NULL
		)
	`

	_, err := s.db.ExecContext(ctx, initUsers)
	if err != nil {
		return err
	}

	initCards := `
		CREATE TABLE IF NOT EXISTS cards (
			ID INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt TEXT NOT NULL,
			answer TEXT NOT NULL,
			user TEXT NOT NULL REFERENCES users(username)
		)
	`
	_, err = s.db.ExecContext(ctx, initCards)
	if err != nil {
		return err
	}

	initTags := `
		CREATE TABLE IF NOT EXISTS tags (
			name TEXT NOT NULL,
			card INTEGER NOT NULL REFERENCES cards(id),
			PRIMARY KEY (name, card)
		)
	`

	_, err = s.db.ExecContext(ctx, initTags)
	if err != nil {
		return err
	}

	initStats := `
		CREATE TABLE IF NOT EXISTS stats (
			user TEXT NOT NULL REFERENCES users(username),
			card INTEGER NOT NULL REFERENCES cards(id),
			next_review DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			current_interval_hours INTEGER NOT NULL CHECK(current_interval_hours > 0) DEFAULT 1,
			PRIMARY KEY(user, card)
		)
	`
	_, err = s.db.ExecContext(ctx, initStats)
	if err != nil {
		return err
	}

	return nil
}

// GetCardsByUser returns all of a user's cards, newest last, with their
// tags attached.
func (s *Store) GetCardsByUser(ctx context.Context, user string) ([]Card, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.prompt, c.answer, c.user,
			GROUP_CONCAT(t.name, ',') AS tags
		FROM cards c
		LEFT JOIN tags t ON t.card = c.id
		WHERE c.user = ?
		GROUP BY c.id
		ORDER BY c.id
	`, user)
	if err != nil {
		return nil, fmt.Errorf("fetch cards: %w", err)
	}
	defer rows.Close()

	var result []Card
	for rows.Next() {
		var card Card
		var tagStr sql.NullString
		if err := rows.Scan(&card.ID, &card.Prompt, &card.Answer, &card.Username, &tagStr); err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		if tagStr.Valid && tagStr.String != "" {
			card.Tags = strings.Split(tagStr.String, ",")
		}
		result = append(result, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read next card: %w", err)
	}
	return result, nil
}

// GetUser fetches a user by their username.
func (s *Store) GetUser(ctx context.Context, username string) (*User, error) {
	var user User
	query := "SELECT username, password FROM users WHERE username = ?"
	row := s.db.QueryRowContext(ctx, query, username)
	err := row.Scan(&user.Username, &user.Password)

	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetAllTags retrieves all unique tags stored in the system.
func (s *Store) GetAllTags(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT name FROM tags")
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err = rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tags: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// CreateUser inserts a new user record.
func (s *Store) CreateUser(ctx context.Context, u User) error {
	query := `INSERT INTO users (username, password) VALUES(?, ?)`
	_, err := s.db.ExecContext(ctx, query, u.Username, u.Password)
	return err
}

// CreateCard creates a new card and associates its tags.
func (s *Store) CreateCard(ctx context.Context, c *Card) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create txn: %w", err)
	}
	defer tx.Rollback()

	// Populate the cards table
	insertCardQuery := `INSERT INTO cards (prompt, answer, user) VALUES(?, ?, ?)`
	res, err := tx.ExecContext(ctx, insertCardQuery, c.Prompt, c.Answer, c.Username)
	if err != nil {
		return fmt.Errorf("insert card: %w", err)
	}
	c.ID, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get insert id: %w", err)
	}

	// Populate the tags table
	for _, tag := range c.Tags {
		insertTagQuery := `INSERT INTO tags (name, card) VALUES(?, ?)`
		if _, err = tx.ExecContext(ctx, insertTagQuery, tag, c.ID); err != nil {
			return fmt.Errorf("insert card tag: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateCard updates an existing card's prompt, answer, and tags.
func (s *Store) UpdateCard(ctx context.Context, c Card) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create txn: %w", err)
	}
	defer tx.Rollback()

	updateCardQuery := `UPDATE cards SET answer = ?, prompt = ? WHERE id = ?`
	_, err = tx.ExecContext(ctx, updateCardQuery, c.Answer, c.Prompt, c.ID)
	if err != nil {
		return fmt.Errorf("update card: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM tags WHERE card = ?", c.ID)
	if err != nil {
		return fmt.Errorf("update card tag: %w", err)
	}

	for _, tag := range c.Tags {
		insertTagQuery := `INSERT INTO tags (name, card) VALUES(?, ?)`
		if _, err = tx.ExecContext(ctx, insertTagQuery, tag, c.ID); err != nil {
			return fmt.Errorf("update card tag: %w", err)
		}
	}

	return tx.Commit()
}

// UpsertCardStat updates or inserts spaced-repetition statistics for a card attempt.
// If correct, doubles the review interval; if incorrect, resets interval to 1 hour.
func (s *Store) UpsertCardStat(ctx context.Context, user string, card int64, isCorrect bool) error {
	const upsertQuery = `
		INSERT INTO stats (user, card, next_review, current_interval_hours)
		VALUES (?, ?,
			datetime('now', '+' || (CASE WHEN ? THEN 2 ELSE 1 END) || ' hours'),
			CASE WHEN ? THEN 2 ELSE 1 END)
		ON CONFLICT(user, card) DO UPDATE SET
			current_interval_hours = CASE WHEN ? THEN current_interval_hours * 2 ELSE 1 END,
			next_review = datetime('now', '+' || (CASE WHEN ? THEN current_interval_hours * 2 ELSE 1 END) || ' hours')
	`
	correct := 0
	if isCorrect {
		correct = 1
	}
	// correct is bound once per ?; all six occurrences mean the same answer.
	_, err := s.db.ExecContext(ctx, upsertQuery, user, card, correct, correct, correct, correct)
	if err != nil {
		return fmt.Errorf("upsert stat: %w", err)
	}

	return nil
}

// DeleteCards deletes cards and their associated stats and tags by IDs.
func (s *Store) DeleteCards(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create txn: %w", err)
	}
	defer tx.Rollback()

	var buffer strings.Builder
	for idx, id := range ids {
		if idx > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(strconv.Itoa(int(id)))
	}

	deleteStatsQuery := fmt.Sprintf(`DELETE FROM stats WHERE card IN (%s)`, buffer.String())
	if _, err = tx.ExecContext(ctx, deleteStatsQuery); err != nil {
		return fmt.Errorf("delete card stats: %w", err)
	}

	deleteTagsQuery := fmt.Sprintf(`DELETE FROM tags WHERE card IN (%s)`, buffer.String())
	if _, err = tx.ExecContext(ctx, deleteTagsQuery); err != nil {
		return fmt.Errorf("delete card tags: %w", err)
	}

	deleteCardsQuery := fmt.Sprintf(`DELETE FROM cards WHERE id IN (%s)`, buffer.String())
	if _, err = tx.ExecContext(ctx, deleteCardsQuery); err != nil {
		return fmt.Errorf("delete cards: %w", err)
	}

	return tx.Commit()
}

// TagCounts returns every tag with total and due-for-review counts for a user.
func (s *Store) TagCounts(ctx context.Context, user string) ([]TagCount, error) {
	const q = `
		SELECT t.name,
		       COUNT(*),
		       SUM(CASE WHEN st.next_review IS NULL OR st.next_review <= datetime('now') 
			   	THEN 1 ELSE 0 END)
		FROM tags t
		LEFT JOIN stats st ON st.card = t.card AND st.user = ?
		GROUP BY t.name
		ORDER BY t.name`
	rows, err := s.db.QueryContext(ctx, q, user)
	if err != nil {
		return nil, fmt.Errorf("tag counts: %w", err)
	}
	defer rows.Close()

	var out []TagCount
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Name, &tc.Total, &tc.Due); err != nil {
			return nil, fmt.Errorf("scan tag count: %w", err)
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// DueCardsByTag returns cards under a tag that are new or due for this user.
func (s *Store) DueCardsByTag(ctx context.Context, user, tag string) ([]Card, error) {
	const q = `
		SELECT c.id, c.prompt, c.answer, c.user
		FROM cards c
		JOIN tags t ON t.card = c.id AND t.name = ?
		LEFT JOIN stats st ON st.card = c.id AND st.user = ?
		WHERE st.next_review IS NULL OR st.next_review <= datetime('now')
		ORDER BY c.id`
	rows, err := s.db.QueryContext(ctx, q, tag, user)
	if err != nil {
		return nil, fmt.Errorf("due cards: %w", err)
	}
	defer rows.Close()

	var out []Card
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.Prompt, &c.Answer, &c.Username); err != nil {
			return nil, fmt.Errorf("scan due card: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SeedDemoData creates the documented demo accounts and sample cards for
// manual testing. Intended for empty dev databases only: it runs through the
// same CreateUser/CreateCard code paths as the app, so passwords are hashed
// identically. It is a no-op if any user already exists.
func (s *Store) SeedDemoData(ctx context.Context) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return false, fmt.Errorf("seed check users: %w", err)
	}
	if n > 0 {
		return false, nil
	}

	for _, u := range []struct {
		name, pw string
		cards    []Card
	}{
		{"demo", "demo1234", []Card{
			{Prompt: "Capital of France?", Answer: "Paris", Tags: []string{"geo", "capitals"}},
			{Prompt: "Largest ocean?", Answer: "Pacific", Tags: []string{"geo"}},
			{Prompt: "2 + 2?", Answer: "4", Tags: []string{"math", "basics"}},
			{Prompt: "H2O is?", Answer: "Water", Tags: []string{"chem", "basics"}},
		}},
		{"reviewer", "reviewer1", []Card{
			{Prompt: "Boiling point of water (C)?", Answer: "100", Tags: []string{"chem"}},
		}},
		{"test", "test1234", []Card{
			{Prompt: "ABC", Answer: "A - Apple\nB - Beetroot\nC - Carrot", Tags: []string{"vegetable"}},
			{Prompt: "What are good for the eyes?", Answer: "Carrots", Tags: []string{"vegetables"}},
		}},
	} {
		hash, err := hashPassword(u.pw)
		if err != nil {
			return false, fmt.Errorf("seed hash %s: %w", u.name, err)
		}
		if err := s.CreateUser(ctx, User{Username: u.name, Password: hash}); err != nil {
			return false, fmt.Errorf("seed user %s: %w", u.name, err)
		}
		for i := range u.cards {
			u.cards[i].Username = u.name
			if err := s.CreateCard(ctx, &u.cards[i]); err != nil {
				return false, fmt.Errorf("seed card for %s: %w", u.name, err)
			}
		}
	}
	return true, nil
}
