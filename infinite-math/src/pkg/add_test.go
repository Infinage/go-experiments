package infimath

import (
	"fmt"
	"testing"
)

func TestGetAdditionProblem(t *testing.T) {
	for i := range 100 {
		t.Run(fmt.Sprintf("Test #%d", i), func(t *testing.T) {
			n1, n2, answer := GetAdditionProblem()
			t.Log(n1, n2, answer)
			if n1 < 1 || n2 < 1 || answer < 1 {
				t.Error("Expected n1, n2 >= 0 answer >= 1")
			} else if n1+n2 != answer {
				t.Error("Expected a problem with n1 + n2 = answer")
			}
		})
	}
}
