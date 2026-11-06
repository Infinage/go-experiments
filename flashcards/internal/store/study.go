package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/infinage/go-experiments/flashcards/internal/srs"
)

// parseTime converts a stored timestamp to a time.Time (zero when empty).
func parseTime(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}

// masteredDays mirrors srs.MasteredDays in SQL filters.
const masteredDays = 21

// timeFmt matches SQLite strftime('%Y-%m-%dT%H:%M:%fZ') output (fixed
// millisecond precision) so due_at string comparisons are exact.
const timeFmt = "2006-01-02T15:04:05.000Z"

// StudyCard is a card in the study queue with its scheduling state.
type StudyCard struct {
	Card
	State srs.State
	// New reports whether the card has never been reviewed.
	New bool
}

// StudyStats summarises the queue for the header boxes.
type StudyStats struct {
	Total      int
	Due        int // includes never-reviewed cards
	Mastered   int
	InProgress int
	NotStarted int
	// NextDue is the next time a card comes due, when nothing is due now.
	NextDue time.Time

	nextDueRaw string
}

// NextDueText renders NextDue relative to now, for the caught-up screen.
func (st StudyStats) NextDueText() string {
	if st.NextDue.IsZero() {
		return ""
	}
	d := time.Until(st.NextDue)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 7*24*time.Hour:
		return "this week"
	default:
		return "later"
	}
}

// Queue returns study cards, oldest due first.
//   - hideMastered filters mastered cards out of the queue.
//   - includeAll drops the due filter ("review anyway"): every card is
//     queued regardless of its due date.
//
// The state row is left-joined: unseen cards are new and due immediately.
func (s *Store) Queue(hideMastered, includeAll bool) ([]StudyCard, error) {
	where := []string{"1=1"}
	if !includeAll {
		where = append(where,
			"rs.card_id IS NULL OR rs.due_at <= strftime('%Y-%m-%dT%H:%M:%fZ','now')")
	}
	if hideMastered {
		where = append(where, "IFNULL(rs.interval_days, 0) < ?")
	}
	args := []any{}
	if hideMastered {
		args = append(args, masteredDays)
	}
	rows, err := s.db.Query(`SELECT `+cardCols+`,
		IFNULL(rs.due_at, ''), IFNULL(rs.interval_days, 0),
		IFNULL(rs.ease, 2.5), IFNULL(rs.repetitions, 0), IFNULL(rs.lapses, 0),
		rs.card_id IS NULL
		FROM cards
		LEFT JOIN review_state rs ON rs.card_id = cards.id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY IFNULL(rs.due_at, '0000') ASC, cards.id ASC`, args...)
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
			&due, &sc.State.Interval, &sc.State.Ease,
			&sc.State.Repetitions, &sc.State.Lapses, &sc.New); err != nil {
			return nil, err
		}
		sc.State.DueAt = parseTime(due)
		out = append(out, sc)
	}
	return out, rows.Err()
}

// CategoryProgress is a deck's mastered/total counts, for the sidebar.
type CategoryProgress struct {
	Name     string
	Total    int
	Mastered int
}

// CategoryProgresses returns per-category progress, alphabetically.
// Uncategorised cards are excluded.
func (s *Store) CategoryProgresses() ([]CategoryProgress, error) {
	rows, err := s.db.Query(`SELECT IFNULL(c.category,''), COUNT(*),
			SUM(CASE WHEN IFNULL(rs.interval_days, 0) >= ? THEN 1 ELSE 0 END)
		FROM cards c
		LEFT JOIN review_state rs ON rs.card_id = c.id
		WHERE IFNULL(c.category,'') <> ''
		GROUP BY c.category ORDER BY c.category`, masteredDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CategoryProgress
	for rows.Next() {
		var cp CategoryProgress
		if err := rows.Scan(&cp.Name, &cp.Total, &cp.Mastered); err != nil {
			return nil, err
		}
		out = append(out, cp)
	}
	return out, rows.Err()
}

// ResetProgress clears scheduling state for a category ("": all cards),
// so they queue up as new again.
func (s *Store) ResetProgress(category string) error {
	if category == "" {
		_, err := s.db.Exec(`DELETE FROM review_state`)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM review_state WHERE card_id IN
		(SELECT id FROM cards WHERE category = ?)`, category)
	return err
}

// Stats returns counts for the study header.
func (s *Store) Stats() (StudyStats, error) {
	var st StudyStats
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&st.Total); err != nil {
		return st, err
	}
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM review_state WHERE interval_days >= ?`,
		masteredDays).Scan(&st.Mastered); err != nil {
		return st, err
	}
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM review_state WHERE interval_days < ?`,
		masteredDays).Scan(&st.InProgress); err != nil {
		return st, err
	}
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM cards c
		 LEFT JOIN review_state rs ON rs.card_id = c.id
		 WHERE rs.card_id IS NULL`).Scan(&st.NotStarted); err != nil {
		return st, err
	}
	err := s.db.QueryRow(`SELECT
			COALESCE(SUM(CASE WHEN rs.card_id IS NULL
				OR rs.due_at <= strftime('%Y-%m-%dT%H:%M:%fZ','now')
				THEN 1 ELSE 0 END), 0),
			COALESCE(MIN(CASE WHEN rs.card_id IS NOT NULL
				AND rs.due_at > strftime('%Y-%m-%dT%H:%M:%fZ','now')
				THEN rs.due_at END), '')
		FROM cards c
		LEFT JOIN review_state rs ON rs.card_id = c.id`).
		Scan(&st.Due, &st.nextDueRaw)
	st.NextDue = parseTime(st.nextDueRaw)
	return st, err
}

// Review applies a grade to a card: updates its scheduling state and
// appends to the review log, in one transaction. Cards with no prior
// state start from srs.New().
func (s *Store) Review(cardID int64, g srs.Grade, now time.Time) (srs.State, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return srs.State{}, err
	}
	defer tx.Rollback()

	cur := srs.New()
	var due string
	err = tx.QueryRow(
		`SELECT due_at, interval_days, ease, repetitions, lapses
		 FROM review_state WHERE card_id = ?`, cardID).Scan(
		&due, &cur.Interval, &cur.Ease, &cur.Repetitions, &cur.Lapses)
	if err != nil && err != sql.ErrNoRows {
		return srs.State{}, err
	}
	cur.DueAt = parseTime(due)
	next := srs.Review(cur, g, now)

	if _, err := tx.Exec(`INSERT INTO review_state
		(card_id, due_at, interval_days, ease, repetitions, lapses)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(card_id) DO UPDATE SET
			due_at=excluded.due_at, interval_days=excluded.interval_days,
			ease=excluded.ease, repetitions=excluded.repetitions,
			lapses=excluded.lapses`,
		cardID, next.DueAt.UTC().Format(timeFmt), next.Interval,
		next.Ease, next.Repetitions, next.Lapses); err != nil {
		return srs.State{}, err
	}
	if _, err := tx.Exec(
		`INSERT INTO review_log(card_id, reviewed_at, grade) VALUES (?,?,?)`,
		cardID, now.UTC().Format(time.RFC3339Nano), int(g)); err != nil {
		return srs.State{}, err
	}
	return next, tx.Commit()
}
