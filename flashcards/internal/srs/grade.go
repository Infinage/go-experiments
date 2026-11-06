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
	case "forgot", "again", "0":
		return Forgot, nil
	case "hard", "1":
		return Hard, nil
	case "medium", "good", "2":
		return Medium, nil
	case "easy", "3":
		return Easy, nil
	}
	return Forgot, fmt.Errorf("invalid grade %q", s)
}

// Label is the human-readable name of a grade.
func (g Grade) Label() string {
	switch g {
	case Forgot:
		return "Forgot"
	case Hard:
		return "Hard"
	case Medium:
		return "Medium"
	case Easy:
		return "Easy"
	}
	return "Unknown(" + strconv.Itoa(int(g)) + ")"
}
