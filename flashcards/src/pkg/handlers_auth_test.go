package flashcards

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// The production 600k PBKDF2 rounds cost ~1.3s per call under the race
	// detector; the hash round-trip properties don't depend on the work factor.
	pbkdf2Iter = 1000
	os.Exit(m.Run())
}

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("hunter2!")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if !checkPassword(hash, "hunter2!") {
		t.Error("checkPassword should accept the original password")
	}
	if checkPassword(hash, "hunter3!") {
		t.Error("checkPassword should reject a wrong password")
	}
}

func TestCheckPasswordMalformedStored(t *testing.T) {
	for _, stored := range []string{"", "noseparator", "bad$hex$!"} {
		if checkPassword(stored, "whatever") {
			t.Errorf("checkPassword(%q) should be false", stored)
		}
	}
}
