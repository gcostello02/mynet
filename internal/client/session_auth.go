package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/control"
)

// SessionRefreshLead is how long before expiry we proactively renew the bearer token.
const SessionRefreshLead = 30 * time.Minute

// ErrDeviceAuthorizationTerminal means the device identity was revoked or denied; re-enrollment is required.
var ErrDeviceAuthorizationTerminal = errors.New("device authorization revoked or denied")

// DeviceAuthSnapshot is safe diagnostic state (no secrets).
type DeviceAuthSnapshot struct {
	RecoveryState       string // stable | renewing | failed | revoked
	SessionExpiresAt    string
	LastRenewalAt       time.Time
	ConsecutiveFailures int
	LastError           string
}

func (r *Runtime) DeviceAuthSnapshot() DeviceAuthSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deviceAuthSnapshotLocked()
}

func (r *Runtime) deviceAuthSnapshotLocked() DeviceAuthSnapshot {
	snap := DeviceAuthSnapshot{
		RecoveryState:       r.authRecoveryState,
		LastRenewalAt:       r.authLastRenewalAt,
		ConsecutiveFailures: r.authRenewFailures,
		LastError:           r.authLastError,
	}
	if r.authTerminalErr != nil {
		snap.RecoveryState = "revoked"
		snap.LastError = r.authTerminalErr.Error()
	}
	if tok, err := config.LoadSessionToken(r.StateDir); err == nil && tok != nil {
		snap.SessionExpiresAt = tok.ExpiresAt
	}
	if snap.RecoveryState == "" {
		if snap.SessionExpiresAt != "" {
			snap.RecoveryState = "stable"
		} else {
			snap.RecoveryState = "missing"
		}
	}
	return snap
}

func (r *Runtime) AuthTerminal() bool {
	r.mu.Lock()
	ok := r.authTerminalErr != nil
	r.mu.Unlock()
	return ok
}

// EnsureDeviceSession renews the bearer token when missing or nearing expiry.
func (r *Runtime) EnsureDeviceSession(ctx context.Context) error {
	if r.AuthTerminal() {
		r.mu.Lock()
		err := r.authTerminalErr
		r.mu.Unlock()
		return err
	}
	if !r.sessionTokenNeedsRefresh() {
		r.mu.Lock()
		has := r.api != nil && r.api.Token != ""
		r.mu.Unlock()
		if has {
			return nil
		}
	}
	return r.refreshDeviceSession(ctx, false)
}

// RecoverDeviceSession forces a new bearer token after an HTTP 401/403 (e.g. expired session).
func (r *Runtime) RecoverDeviceSession(ctx context.Context) error {
	if r.AuthTerminal() {
		r.mu.Lock()
		err := r.authTerminalErr
		r.mu.Unlock()
		return err
	}
	return r.refreshDeviceSession(ctx, true)
}

func (r *Runtime) sessionTokenNeedsRefresh() bool {
	tok, err := config.LoadSessionToken(r.StateDir)
	if err != nil || tok == nil || tok.Token == "" {
		return true
	}
	if tok.ExpiresAt == "" {
		return false
	}
	exp, err := time.Parse(time.RFC3339, tok.ExpiresAt)
	if err != nil {
		return false
	}
	return !time.Now().UTC().Before(exp.Add(-SessionRefreshLead))
}

func (r *Runtime) refreshDeviceSession(ctx context.Context, forced bool) error {
	r.authRefreshMu.Lock()
	defer r.authRefreshMu.Unlock()

	if r.AuthTerminal() {
		r.mu.Lock()
		err := r.authTerminalErr
		r.mu.Unlock()
		return err
	}

	r.mu.Lock()
	r.authRecoveryState = "renewing"
	r.mu.Unlock()

	if err := r.load(); err != nil {
		r.noteAuthRenewFailure(err)
		return err
	}
	if r.identity == nil {
		err := errors.New("device identity missing")
		r.noteAuthRenewFailure(err)
		return err
	}

	type authResult struct {
		resp control.AuthResponse
		err  error
	}
	done := make(chan authResult, 1)
	go func() {
		resp, err := r.api.Auth(r.identity)
		done <- authResult{resp, err}
	}()

	var res authResult
	select {
	case <-ctx.Done():
		err := ctx.Err()
		r.noteAuthRenewFailure(err)
		return err
	case res = <-done:
	}

	if res.err != nil {
		if isDeviceAuthTerminal(res.err) {
			r.markAuthTerminal(res.err)
			_ = r.ClearDeviceSession()
			return ErrDeviceAuthorizationTerminal
		}
		r.noteAuthRenewFailure(res.err)
		return res.err
	}

	if err := config.SaveSessionToken(r.StateDir, config.SessionToken{
		Token:     res.resp.Token,
		ExpiresAt: res.resp.ExpiresAt,
	}); err != nil {
		r.noteAuthRenewFailure(err)
		return err
	}
	r.mu.Lock()
	if r.api != nil {
		r.api.Token = res.resp.Token
	}
	r.connState = StateRegistered
	r.authRenewFailures = 0
	r.authLastError = ""
	r.authRecoveryState = "stable"
	r.authLastRenewalAt = time.Now().UTC()
	r.mu.Unlock()

	if r.Logger != nil {
		if forced {
			r.Logger.Info("device session renewed after auth failure")
		} else {
			r.Logger.Debug("device session renewed")
		}
	}
	r.refreshICEServers()
	return nil
}

func (r *Runtime) noteAuthRenewFailure(err error) {
	r.mu.Lock()
	r.authRenewFailures++
	r.authLastError = err.Error()
	if r.authRenewFailures >= 12 {
		r.authRecoveryState = "failed"
	} else {
		r.authRecoveryState = "failed"
	}
	r.mu.Unlock()
}

func (r *Runtime) markAuthTerminal(err error) {
	r.mu.Lock()
	r.authTerminalErr = fmt.Errorf("%w: %v", ErrDeviceAuthorizationTerminal, err)
	r.authRecoveryState = "revoked"
	r.authLastError = r.authTerminalErr.Error()
	r.mu.Unlock()
	if r.Logger != nil {
		r.Logger.Info("device authorization terminal", "err", err)
	}
}

func isDeviceAuthTerminal(err error) bool {
	// POST /v1/auth failures mean the device identity is no longer accepted.
	return control.IsHTTPStatus(err, 401) || control.IsHTTPStatus(err, 403)
}

func isRecoverableHTTPAuth(err error) bool {
	return control.IsHTTPStatus(err, 401) || control.IsHTTPStatus(err, 403)
}

func isWebSocketAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "401") ||
		strings.Contains(s, "403") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "invalid token")
}
