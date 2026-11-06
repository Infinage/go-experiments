package srs

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseGrade converts a request parameter ("again"|"hard"|"good"|"easy"
// or 0-3) to a Grade.
func ParseGrade(s string) (Grade, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "again", "0":
		return Again, nil
	case "hard", "1":
		return Hard, nil
	case "good", "2":
		return Good, nil
	case "easy", "3":
		return Easy, nil
	}
	return Again, fmt.Errorf("invalid grade %q", s)
}

// Label is the human-readable name of a grade.
func (g Grade) Label() string {
	switch g {
	case Again:
		return "Again"
	case Hard:
		return "Hard"
	case Good:
		return "Good"
	case Easy:
		return "Easy"
	}
	return "Unknown(" + strconv.Itoa(int(g)) + ")"
}
