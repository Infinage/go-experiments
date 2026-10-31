package flashcards

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

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
			user TEXT NOT NULL REFEFRENCES(users(username))
		)
	`
	_, err = s.db.ExecContext(ctx, initCards)
	if err != nil {
		return err
	}

	initTags := `
		CREATE TABLE IF NOT EXISTS tags (
			name TEXT NOT NULL,
			card INTEGER NOT NULL REFEFRENCES(card(id)),
			PRIMARY KEY (name, card)
		)
	`

	_, err = s.db.ExecContext(ctx, initTags)
	if err != nil {
		return err
	}

	initStats := `
		CREATE TABLE IF NOT EXISTS stats (
			user TEXT NOT NULL REFEFRENCES(users(username)),
			card INTEGER NOT NULL REFEFRENCES(cards(id)),
			next_review DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			current_interval_hours INTEGER NOT NULL CHECK(current_interval_hours > 0) DEFAULT 1
			PRIMARY KEY(user, card)
		)
	`
	_, err = s.db.ExecContext(ctx, initStats)
	if err != nil {
		return err
	}

	return nil
}

// GetAllCards returns all the cards, if userId != "" filters out cards 
// that have been attempted and not requiring re-review at the moment.
func (s *Store) GetAllCards(ctx context.Context, userId string) ([]Card, error) {
	var data []any
	fetchCardQuery := `
		SELECT 
			c.id, c.prompt, c.answer, c.user, 
			GROUP_CONCAT(t.name, ',') AS tags
		FROM cards c
		LEFT JOIN tags t ON t.card = c.id
		GROUP BY c.id
	`

	if userId != "" {
		data = append(data, userId)
		fetchCardQuery = `
			SELECT 
				c.id, c.prompt, c.answer, c.user, 
				GROUP_CONCAT(t.name, ',') AS tags
			FROM cards c
			LEFT JOIN tags t ON t.card = c.id
			LEFT JOIN stats s ON s.user = ?
			WHERE s.next_review == null || s.next_review <= CURRENT_TIMESTAMP
			GROUP BY c.id
		`
	}

	rows, err := s.db.QueryContext(ctx, fetchCardQuery, data...)
	if err != nil {
		return nil, fmt.Errorf("fetch cards: %w", err)
	}

	var result []Card
	for rows.Next() {
		var card Card
		var tagStr string
		err = rows.Scan(&card.ID, &card.Prompt, &card.Answer, &card.Username, &tagStr)
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		card.Tags = strings.Split(tagStr, ",")
		result = append(result, card)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("read next card: %w", err)
	}

	return result, nil
}

func (s *Store) GetUser(ctx context.Context, username string) (*User, error) {
	var user User
	query := "SELECT username, password from users where id = ?"
	row := s.db.QueryRowContext(ctx, query, username)
	err := row.Scan(&user.Username, &user.Password)
	return &user, err
}

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

func (s *Store) CreateUser(ctx context.Context, u User) error {
	query := `INSERT INTO users (username, password) values(?, ?)`
	_, err := s.db.ExecContext(ctx, query, u.Username, u.Password)
	return err
}

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
		if _, err = tx.ExecContext(ctx, insertTagQuery, c.ID, tag); err != nil {
			return fmt.Errorf("insert card tag: %w", err)
		}
	}

	return tx.Commit()
}

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

	_, err = tx.ExecContext(ctx, "DELETE * FROM tags WHERE card = ?", c.ID)
	if err != nil {
		return fmt.Errorf("update card tag: %w", err)
	}

	for _, tag := range c.Tags {
		insertTagQuery := `INSERT INTO tags (name, card) VALUES(?, ?)`
		if _, err = tx.ExecContext(ctx, insertTagQuery, c.ID, tag); err != nil {
			return fmt.Errorf("update card tag: %w", err)
		}
	}

	return tx.Commit()
}

func (s *Store) UpsertCardStat(ctx context.Context, user string, card int64, 
	isCorrect bool) {

	var stat CardStat
}

func (s *Store) DeleteCards(ctx context.Context, ids []int64) error {
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

	deleteTagsQuery := fmt.Sprintf(`DELETE FROM tags WHERE card IN (%s)`, buffer.String())
	_, err = tx.ExecContext(ctx, deleteTagsQuery, nil)
	if err != nil {
		return fmt.Errorf("delete card tags: %w", err)
	}

	deleteCardsQuery := fmt.Sprintf(`DELETE FROM cards WHERE ID IN (%s)`, buffer.String())
	_, err = tx.ExecContext(ctx, deleteCardsQuery, nil)
	if err != nil {
		return fmt.Errorf("delete cards: %w", err)
	}

	return tx.Commit()
}
