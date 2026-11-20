package infimath

import "math/rand/v2"

func GetAdditionProblem() (n1 int, n2 int, answer int) {
	n1 = rand.IntN(99) + 1
	answer = rand.IntN(99) + 1

	// If res == n1, n2 would be 0
	n1, answer = min(n1, answer), max(n1, answer)
	if n1 == answer {
		if answer == 1 {
			answer++
		} else {
			n1--
		}
	}

	return n1, answer - n1, answer
}
