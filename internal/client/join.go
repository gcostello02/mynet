package client

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/identity"
	"github.com/gcostello02/mynet/internal/invitation"
	"github.com/gcostello02/mynet/internal/join"
)

// JoinOptions configures secure network join.
type JoinOptions struct {
	DeviceName string
	Force      bool // overwrite existing identity/config
}

// Join verifies the coordinator, enrolls with the invitation, and authenticates the device.
// Does not install launchd/systemd or start VPN (run mynet service install / mynet up separately).
func (r *Runtime) Join(inv invitation.Invite, opts JoinOptions) error {
	if err := inv.Validate(); err != nil {
		return err
	}
	if err := join.VerifyCoordinator(inv); err != nil {
		return err
	}
	name := opts.DeviceName
	if name == "" {
		return errors.New("device name is required (--name)")
	}
	idPath := config.IdentityPath(r.StateDir)
	if _, err := os.Stat(idPath); err == nil && !opts.Force {
		return fmt.Errorf("device identity already exists at %s (use --force to replace or join is single-network only in V0.4)", idPath)
	}
	if opts.Force {
		_ = os.Remove(idPath)
	}
	id, err := identity.Generate(name)
	if err != nil {
		return err
	}
	if err := identity.Save(idPath, id); err != nil {
		return err
	}
	cfg, _ := config.LoadClientConfig(r.StateDir)
	cfg.ServerURL = inv.ServerURL
	cfg.NetworkID = inv.NetworkID
	cfg.CoordinatorFingerprint = inv.CoordinatorFingerprint
	cfg.CoordinatorPublicKey = inv.CoordinatorPublicKey
	if cfg.CoordinatorPublicKey == "" {
		api := control.NewSecureAPIClient(inv.ServerURL)
		pub, err := api.NetworkPublic()
		if err != nil {
			return err
		}
		cfg.CoordinatorPublicKey = pub.CoordinatorPublicKey
	}
	if err := config.SaveClientConfig(r.StateDir, cfg); err != nil {
		return err
	}
	r.identity = id
	r.cfg = cfg
	r.api = control.NewSecureAPIClient(inv.ServerURL)

	if err := r.api.EnrollWithInvitation(id, inv.EnrollmentToken, inv.CoordinatorFingerprint); err != nil {
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "already") {
			// Device may already be enrolled; continue to auth.
		} else {
			return fmt.Errorf("enroll: %w", err)
		}
	}
	resp, err := r.api.Auth(id)
	if err != nil {
		if isRevokedOrDenied(err) {
			_ = config.ClearSessionToken(r.StateDir)
		}
		return fmt.Errorf("auth: %w", err)
	}
	if err := config.SaveSessionToken(r.StateDir, config.SessionToken{
		Token:     resp.Token,
		ExpiresAt: resp.ExpiresAt,
	}); err != nil {
		return err
	}
	r.mu.Lock()
	r.connState = StateRegistered
	r.mu.Unlock()
	if _, err := r.RegisterVPN(); err != nil {
		return fmt.Errorf("vpn register: %w", err)
	}
	return nil
}

func isRevokedOrDenied(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "401") || strings.Contains(s, "403") || strings.Contains(s, "revoked")
}

// ClearDeviceSession removes device bearer credentials after revocation.
func (r *Runtime) ClearDeviceSession() error {
	if err := config.ClearSessionToken(r.StateDir); err != nil {
		return err
	}
	r.mu.Lock()
	if r.api != nil {
		r.api.Token = ""
	}
	r.connState = StateDisconnected
	r.mu.Unlock()
	return nil
}
