package flashcards

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const (
	cookieName = "sid"
	sessionTTL = 7 * 24 * time.Hour
)

// pbkdf2Iter is a var only so tests can use a cheap work factor
var pbkdf2Iter = 600_000

type authedHandler func(http.ResponseWriter, *http.Request, *session)

// authed shows the login form (via SSE patch) when there's no valid session.
func (s *Server) authed(h authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := s.lookup(r)
		if sess == nil {
			s.patchMain(w, r, "login", map[string]any{})
			return
		}
		h(w, r, sess)
	}
}

// ---------- sessions ----------

func (s *Server) lookup(r *http.Request) *session {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.sessions[c.Value]
	if sess == nil {
		return nil
	}
	if time.Now().After(sess.expires) {
		delete(s.sessions, c.Value)
		return nil
	}
	return sess
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)

	s.mu.Lock()
	s.sessions[tok] = &session{user: user, expires: time.Now().Add(sessionTTL)}
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: tok, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil, MaxAge: int(sessionTTL.Seconds()),
	})
}

// ---------- passwords ----------

// hashPassword derives a PBKDF2 hash from a unique random salt per password.
// The salt is not a secret: it exists to prevent precomputed rainbow-table
// attacks, so storing it alongside the key is the standard, safe approach.
func hashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(key), nil
}

func checkPassword(stored, pw string) bool {
	saltHex, keyHex, ok := strings.Cut(stored, "$")
	if !ok {
		return false
	}
	salt, err1 := hex.DecodeString(saltHex)
	want, err2 := hex.DecodeString(keyHex)
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// ---------- handlers: auth ----------

type authSignals struct {
	Username string
	Password string
}

// handleLogin is called when the user submits the login form (POST /login).
// It verifies credentials, starts a session, and patches the home view on
// success or the login form with an error message on failure.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var sg authSignals
	if err := readSignals(r, &sg); err != nil {
		s.patchMain(w, r, "login", map[string]any{"Error": "Bad request."})
		return
	}
	u, err := s.store.GetUser(r.Context(), strings.TrimSpace(sg.Username))
	if err != nil || !checkPassword(u.Password, sg.Password) {
		s.patchMain(w, r, "login", map[string]any{"Error": "Wrong username or password."})
		return
	}
	s.startSession(w, r, u.Username)
	s.renderHome(w, r, u.Username, "")
}

// handleSignup is called when the user submits the signup form (POST /signup).
// It validates input, hashes the password, creates the user in the DB, and
// starts a session on success (or patches the login form with an error).
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var sg authSignals
	if err := readSignals(r, &sg); err != nil {
		s.patchMain(w, r, "login", map[string]any{"Error": "Bad request."})
		return
	}
	name := strings.TrimSpace(sg.Username)
	switch {
	case name == "" || len(name) > 32:
		s.patchMain(w, r, "login", map[string]any{"Error": "Username must be 1-32 characters."})
		return
	case len(sg.Password) < 8:
		s.patchMain(w, r, "login", map[string]any{"Error": "Password must be at least 8 characters."})
		return
	}
	hash, err := hashPassword(sg.Password)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.store.CreateUser(r.Context(), User{Username: name, Password: hash}); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			s.patchMain(w, r, "login", map[string]any{"Error": "That username is taken."})
		} else {
			fail(w, err)
		}
		return
	}
	s.startSession(w, r, name)
	s.renderHome(w, r, name, "Welcome! Add your first card to get going.")
}

// handleLogout is called when the user clicks "Log out" on the home page
// (POST /logout). It invalidates the session cookie and patches back to the
// login form.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, _ *session) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1, HttpOnly: true})
	s.patchMain(w, r, "login", map[string]any{})
}
