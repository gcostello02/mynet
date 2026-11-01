package client

import (
	"errors"
	"fmt"
	"os"

	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/identity"
	"github.com/gcostello02/mynet/internal/invitation"
	"github.com/gcostello02/mynet/internal/join"
	"github.com/gcostello02/mynet/internal/wgkeys"
)

// JoinRecoveryHint is printed when join fails mid-flight.
const JoinRecoveryHint = "If enrollment may have succeeded, run: mynet login (legacy) or re-join with a new invitation; do not share the invitation."

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
		return fmt.Errorf("device identity already exists (use --force to replace; V0.4 supports one network per device)")
	}
	if opts.Force {
		_ = os.Remove(idPath)
		_ = config.ClearSessionToken(r.StateDir)
		_ = wgkeys.Clear(r.StateDir)
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
		api, err := control.NewSecureAPIClient(inv.ServerURL)
		if err != nil {
			return err
		}
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
	api, err := control.NewSecureAPIClient(inv.ServerURL)
	if err != nil {
		return err
	}
	r.api = api

	enrollErr := r.api.EnrollWithInvitation(id, inv.EnrollmentToken, inv.CoordinatorFingerprint)
	if enrollErr != nil {
		if control.IsHTTPStatus(enrollErr, 409) {
			// 409 alone does not mean success; only continue if this device authenticates.
			if err := r.authenticateAfterEnroll(id); err != nil {
				return fmt.Errorf("enroll conflict and authentication failed: %w (%s)", err, JoinRecoveryHint)
			}
			if _, err := r.RegisterVPN(); err != nil {
				return fmt.Errorf("vpn register: %w (%s)", err, JoinRecoveryHint)
			}
			return nil
		}
		return fmt.Errorf("enroll failed: %w (%s)", enrollErr, JoinRecoveryHint)
	}
	if err := r.authenticateAfterEnroll(id); err != nil {
		return fmt.Errorf("auth failed: %w (%s)", err, JoinRecoveryHint)
	}
	if _, err := r.RegisterVPN(); err != nil {
		return fmt.Errorf("vpn register: %w (%s)", err, JoinRecoveryHint)
	}
	return nil
}

func (r *Runtime) authenticateAfterEnroll(id *identity.DeviceIdentity) error {
	resp, err := r.api.Auth(id)
	if err != nil {
		if isRevokedOrDenied(err) {
			_ = config.ClearSessionToken(r.StateDir)
		}
		return err
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
	return nil
}

func isRevokedOrDenied(err error) bool {
	if err == nil {
		return false
	}
	return control.IsHTTPStatus(err, 401) || control.IsHTTPStatus(err, 403)
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
