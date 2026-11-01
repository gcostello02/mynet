package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	adminSessionTTL  = 12 * time.Hour
	adminCookieName  = "mynet_admin_session"
	adminCSRFCookie  = "mynet_csrf"
	adminCSRFHeader  = "X-Mynet-CSRF"
	setupTokenHeader = "X-Mynet-Setup-Token"
	argonTime        = 2
	argonMemory      = 64 * 1024
	argonThreads     = 4
	argonKeyLen      = 32
)

type AdminRecord struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type AdminSession struct {
	ID        string
	AdminID   string
	CSRFToken string // plaintext for cookie; hash stored in DB
	ExpiresAt time.Time
}

type adminSessionTable struct {
	mu       sync.RWMutex
	sessions map[string]*AdminSession // session id -> session
}

func newAdminSessionTable() *adminSessionTable {
	return &adminSessionTable{sessions: make(map[string]*AdminSession)}
}

func hashAdminPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("argon2id$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyAdminPassword(password, encoded string) bool {
	parts := splitArgonEncoded(encoded)
	if parts == nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts.salt)
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts.key)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

type argonParts struct {
	salt, key string
}

func splitArgonEncoded(encoded string) *argonParts {
	if len(encoded) < 10 || encoded[:9] != "argon2id$" {
		return nil
	}
	rest := encoded[9:]
	i := indexOf(rest, "$")
	if i < 0 {
		return nil
	}
	return &argonParts{salt: rest[:i], key: rest[i+1:]}
}

func hashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte("mynet-invitation-v1\n" + token))
	return hex.EncodeToString(sum[:])
}

func hashCSRFToken(token string) string {
	sum := sha256.Sum256([]byte("mynet-csrf-v1\n" + token))
	return hex.EncodeToString(sum[:])
}

func newRandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (t *adminSessionTable) create(adminID string) (*AdminSession, error) {
	sid, err := newRandomToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := newRandomToken(32)
	if err != nil {
		return nil, err
	}
	sess := &AdminSession{
		ID:        sid,
		AdminID:   adminID,
		CSRFToken: csrf,
		ExpiresAt: time.Now().UTC().Add(adminSessionTTL),
	}
	t.mu.Lock()
	t.sessions[sid] = sess
	t.mu.Unlock()
	return sess, nil
}

func (t *adminSessionTable) get(id string) (*AdminSession, bool) {
	t.mu.RLock()
	s, ok := t.sessions[id]
	t.mu.RUnlock()
	if !ok || time.Now().UTC().After(s.ExpiresAt) {
		return nil, false
	}
	return s, true
}

func (t *adminSessionTable) delete(id string) {
	t.mu.Lock()
	delete(t.sessions, id)
	t.mu.Unlock()
}

// deleteAll removes every admin session (e.g. after password rotation).
func (t *adminSessionTable) deleteAll() {
	t.mu.Lock()
	t.sessions = make(map[string]*AdminSession)
	t.mu.Unlock()
}

var errAdminUnauthorized = errors.New("admin unauthorized")
