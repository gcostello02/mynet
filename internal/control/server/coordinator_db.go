package server

import (
	"errors"
	"fmt"
)

func attachCoordinatorDB(store *Store, path string) error {
	db, err := openCoordinatorDB(path)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	meta, err := loadNetworkMeta(db)
	if err != nil {
		db.Close()
		return fmt.Errorf("load meta: %w", err)
	}
	store.sqlDB = db
	store.dbPath = path
	store.meta = meta

	if err := LoadDevicesFromSQLite(path, store); err != nil {
		return fmt.Errorf("load devices: %w", err)
	}
	admins, err := loadAdmins(db)
	if err != nil {
		return fmt.Errorf("load admins: %w", err)
	}
	store.admins = admins
	store.adminByUser = make(map[string]string, len(admins))
	for _, a := range admins {
		store.adminByUser[a.Username] = a.ID
	}
	if err := loadInvitations(db, store.invitations); err != nil {
		return fmt.Errorf("load invitations: %w", err)
	}
	return nil
}

func (s *Server) verifyDeviceBearer(token string) (string, error) {
	claims, err := decodeSessionClaims(token, s.cfg.ServerSecret)
	if err != nil {
		return "", err
	}
	if _, ok := s.store.GetDevice(claims.DeviceID); !ok {
		return "", errors.New("unknown device")
	}
	epoch := s.store.DeviceAuthEpoch(claims.DeviceID)
	if claims.AuthEpoch != epoch {
		return "", errors.New("token revoked")
	}
	return claims.DeviceID, nil
}
