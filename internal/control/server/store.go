package server

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type DeviceRecord struct {
	DeviceID    string
	Name        string
	PublicKey   string // Ed25519 enrollment key
	NetworkID   string
	WGPublicKey string // WireGuard public key (hex)
	VPNIPv4     string // allocated virtual IPv4, e.g. 10.77.0.2
	Revoked     bool
	AuthEpoch   int
	Enrolled    time.Time
	RenamedAt   time.Time
}

type SessionRecord struct {
	ID        string
	Initiator string
	Responder string
	NetworkID string
	Created   time.Time
}

type Store struct {
	mu sync.RWMutex

	networkID            string
	enrollmentSecretHash string
	dbPath               string
	sqlDB                *sql.DB

	meta          *NetworkMeta
	admins        map[string]*AdminRecord
	adminByUser   map[string]string
	invitations   *invitationStore
	adminSessions *adminSessionTable

	devices  map[string]*DeviceRecord
	byName   map[string]string // name -> device id (per network)
	sessions map[string]*SessionRecord
}

func NewStore(networkID, enrollmentSecretHash string) *Store {
	return &Store{
		networkID:            networkID,
		enrollmentSecretHash: enrollmentSecretHash,
		meta:                 &NetworkMeta{},
		admins:               make(map[string]*AdminRecord),
		adminByUser:          make(map[string]string),
		invitations:          newInvitationStore(),
		adminSessions:        newAdminSessionTable(),
		devices:              make(map[string]*DeviceRecord),
		byName:               make(map[string]string),
		sessions:             make(map[string]*SessionRecord),
	}
}

func (s *Store) EnrollDevice(rec *DeviceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec.Revoked {
		return errRevoked
	}
	if existing, ok := s.devices[rec.DeviceID]; ok {
		if existing.Revoked {
			return errRevoked
		}
		if existing.PublicKey != rec.PublicKey {
			return errDeviceExists
		}
		// Re-enrollment with same keys updates name if changed.
		if other, ok := s.byName[rec.Name]; ok && other != rec.DeviceID {
			return errNameTaken
		}
		delete(s.byName, existing.Name)
		existing.Name = rec.Name
		s.byName[rec.Name] = rec.DeviceID
		if s.dbPath != "" {
			_ = PersistDevice(s.dbPath, existing)
		}
		return nil
	}
	if _, ok := s.byName[rec.Name]; ok {
		return errNameTaken
	}
	s.devices[rec.DeviceID] = rec
	s.byName[rec.Name] = rec.DeviceID
	if s.dbPath != "" {
		_ = PersistDevice(s.dbPath, rec)
	}
	return nil
}

func (s *Store) GetDevice(deviceID string) (*DeviceRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[deviceID]
	if !ok || d.Revoked {
		return nil, false
	}
	return d, ok
}

func (s *Store) GetDeviceByName(name string) (*DeviceRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byName[name]
	if !ok {
		return nil, false
	}
	d, ok := s.devices[id]
	if !ok || d.Revoked {
		return nil, false
	}
	return d, true
}

func (s *Store) ListPeers(networkID, excludeDevice string) []DeviceRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []DeviceRecord
	for _, d := range s.devices {
		if d.Revoked || d.NetworkID != networkID || d.DeviceID == excludeDevice {
			continue
		}
		out = append(out, *d)
	}
	return out
}

func (s *Store) RevokeDevice(deviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[deviceID]
	if !ok || d.Revoked {
		return false
	}
	d.Revoked = true
	d.AuthEpoch++
	delete(s.byName, d.Name)
	if s.dbPath != "" {
		_ = persistDeviceRevoked(s.dbPath, d)
	}
	return true
}

func (s *Store) RenameDevice(deviceID, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[deviceID]
	if !ok || d.Revoked {
		return fmt.Errorf("device not found")
	}
	if other, ok := s.byName[newName]; ok && other != deviceID {
		return errNameTaken
	}
	delete(s.byName, d.Name)
	d.Name = newName
	d.RenamedAt = time.Now().UTC()
	s.byName[newName] = deviceID
	if s.dbPath != "" {
		_ = PersistDevice(s.dbPath, d)
	}
	return nil
}

func (s *Store) ListAllDevices() []DeviceRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DeviceRecord, 0, len(s.devices))
	for _, d := range s.devices {
		out = append(out, *d)
	}
	return out
}

func (s *Store) GetDeviceIncludingRevoked(deviceID string) (*DeviceRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[deviceID]
	if !ok {
		return nil, false
	}
	return d, true
}

func (s *Store) DeviceAuthEpoch(deviceID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[deviceID]
	if !ok {
		return -1
	}
	return d.AuthEpoch
}

func (s *Store) Meta() *NetworkMeta {
	return s.meta
}

func (s *Store) AdminCount() int {
	return len(s.admins)
}

func (s *Store) CreateSession(id, initiator, responder, networkID string) *SessionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := &SessionRecord{
		ID:        id,
		Initiator: initiator,
		Responder: responder,
		NetworkID: networkID,
		Created:   time.Now().UTC(),
	}
	s.sessions[id] = rec
	return rec
}

func (s *Store) GetSession(id string) (*SessionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *Store) DeleteSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *Store) EnrollmentSecretHash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enrollmentSecretHash
}

func (s *Store) NetworkID() string {
	return s.networkID
}

var (
	errDeviceExists = &storeError{"device already enrolled with different keys"}
	errNameTaken    = &storeError{"device name already taken"}
	errRevoked      = &storeError{"device revoked"}
)

type storeError struct{ msg string }

func (e *storeError) Error() string { return e.msg }
