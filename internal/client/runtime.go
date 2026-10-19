package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/control"
	icepkg "github.com/grantcostello/mynet/internal/ice"
	"github.com/grantcostello/mynet/internal/identity"
	"github.com/grantcostello/mynet/internal/peer"
)

// ConnState describes high-level peer connectivity.
type ConnState string

const (
	StateDisconnected ConnState = "Disconnected"
	StateRegistered   ConnState = "Registered"
	StateOnline       ConnState = "Online"
	StateConnecting   ConnState = "Connecting"
	StateConnected    ConnState = "Connected"
	StateFailed       ConnState = "Connection failed"
)

// Runtime coordinates control plane and ICE data plane for one device.
type Runtime struct {
	StateDir string
	Logger   *slog.Logger

	mu sync.Mutex

	identity *identity.DeviceIdentity
	cfg      config.ClientConfig
	api      *control.APIClient

	ws       *websocket.Conn
	wsCancel context.CancelFunc

	activeSession        string
	peerName             string
	connState            ConnState
	icePath              icepkg.PathInfo
	dataConn             net.Conn
	iceSession           interface{ Close() error }
	lastRTT              time.Duration
	lastErr              string
	lastErrAt            time.Time
	lastErrPeer          string
	lastErrKind          string
	lastErrSession       string
	lastHistoricalErr    string
	lastHistoricalErrAt  time.Time
	lastSuccessAt        time.Time
	lastSuccessPeer      string
	lastPingAt           time.Time
	lastLocalCandidates  []icepkg.CandidateInfo
	lastRemoteCandidates []icepkg.CandidateInfo
	connectOpts          ConnectOptions

	connectWait  chan error // initiator waits for ICE result
	iceCancel    context.CancelFunc
	iceCancelFor string

	incomingICE     map[string]bool
	peerInFlightICE map[string]string // peer name -> session ID currently negotiating (VPN + app ICE)

	peerICELockMu    sync.Mutex
	peerICEHandleMu  map[string]*sync.Mutex

	signalMu       sync.Mutex
	signalHandlers map[string]func(control.WSMessage)
	pendingSignals map[string][]control.WSMessage

	serverICE []control.ICEServer

	vpnMode       bool
	peerLinks     map[string]*PeerLink
	peerLinkGen   uint64
	vpnWait       map[string]chan error
	vpnConnecting map[string]bool

	peerOnlineLast map[string]bool // last known coordinator online bit per peer name

	peerICEMu              sync.Mutex
	peerICEGen             uint64
	peerICECancel          map[string]peerICEAttempt
	peerICESupersedeAllow  map[string]bool // set by cancelPeerICEAttempt before CreateSession (initiator)

	signalingMu         sync.Mutex
	signalingRunning    bool
	signalingConnecting bool
	lastWSDisconnect    string
	reconnectAttempts   int
	lastWSMessageType   string
	lastWSMessageAt     time.Time

	lastGatherReport icepkg.GatherReport

	connTiming *connTiming
	recovery       *recoveryTracker
	signalingReadyAt time.Time
}

// APIClient returns the coordination HTTP client (session token if logged in).
func (r *Runtime) APIClient() *control.APIClient {
	return r.api
}

func NewRuntime(stateDir string, log *slog.Logger) *Runtime {
	if log == nil {
		log = slog.Default()
	}
	if stateDir == "" {
		stateDir = config.DefaultStateDir()
	}
	return &Runtime{
		StateDir:       stateDir,
		Logger:         log,
		connState:      StateDisconnected,
		signalHandlers: make(map[string]func(control.WSMessage)),
		pendingSignals: make(map[string][]control.WSMessage),
		incomingICE:     make(map[string]bool),
		peerInFlightICE: make(map[string]string),
		peerOnlineLast:  make(map[string]bool),
		connTiming:     newConnTiming(),
		recovery:       newRecoveryTracker(),
	}
}

func (r *Runtime) Init(name string) error {
	id, err := identity.Generate(name)
	if err != nil {
		return err
	}
	if err := identity.Save(config.IdentityPath(r.StateDir), id); err != nil {
		return err
	}
	cfg := config.DefaultClientConfig()
	if err := config.SaveClientConfig(r.StateDir, cfg); err != nil {
		return err
	}
	r.identity = id
	r.cfg = cfg
	return nil
}

// LoadSessionOnly loads identity/config and session token without opening signaling.
func (r *Runtime) LoadSessionOnly() error {
	return r.load()
}

func (r *Runtime) load() error {
	id, err := identity.Load(config.IdentityPath(r.StateDir))
	if err != nil {
		return fmt.Errorf("identity not found, run mynet init: %w", err)
	}
	cfg, err := config.LoadClientConfig(r.StateDir)
	if err != nil {
		return err
	}
	r.identity = id
	r.cfg = cfg
	r.api = control.NewAPIClient(cfg.ServerURL)
	tok, err := config.LoadSessionToken(r.StateDir)
	if err == nil && tok.Token != "" {
		r.api.Token = tok.Token
		if r.connState == StateDisconnected {
			r.connState = StateRegistered
		}
	}
	return nil
}

// Login enrolls (if needed) and authenticates, then maintains signaling websocket.
func (r *Runtime) Login(enrollmentSecret string) error {
	if err := r.load(); err != nil {
		return err
	}
	if enrollmentSecret != "" {
		if err := r.api.Enroll(r.identity, enrollmentSecret); err != nil {
			if !strings.Contains(err.Error(), "already") && !strings.Contains(err.Error(), "409") {
				return fmt.Errorf("enroll: %w", err)
			}
		}
	}
	resp, err := r.api.Auth(r.identity)
	if err != nil {
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
	r.refreshICEServers()
	return nil
}

func (r *Runtime) refreshICEServers() {
	if r.api == nil {
		return
	}
	if iceResp, err := r.api.ICEServers(); err == nil {
		r.serverICE = iceResp.ICEServers
	} else {
		r.Logger.Debug("ice-servers fetch failed", "err", err)
	}
}

func (r *Runtime) iceServersForSession() []control.ICEServer {
	return config.FilterICEServers(r.cfg, config.BuildICEServers(r.cfg, r.serverICE))
}

// ensureWebSocket connects signaling for short-lived CLI use when no persistent loop runs.
// VPN daemon code should use StartSignaling instead.
func (r *Runtime) ensureWebSocket() error {
	r.signalingMu.Lock()
	running := r.signalingRunning
	r.signalingMu.Unlock()
	if running {
		return r.waitForSignaling(context.Background())
	}
	r.mu.Lock()
	if r.ws != nil {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()
	conn, err := r.api.ConnectWebSocket()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.ws = conn
	r.wsCancel = cancel
	r.connState = StateOnline
	r.mu.Unlock()
	go r.wsReadLoop(ctx, conn)
	return nil
}

func (r *Runtime) wsReadLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		if ctx.Err() != nil {
			return
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			r.mu.Lock()
			r.ws = nil
			if r.wsCancel != nil {
				r.wsCancel()
			}
			if r.dataConn == nil {
				if r.connState == StateOnline || r.connState == StateConnecting {
					r.connState = StateRegistered
				}
			}
			r.mu.Unlock()
			return
		}
		var msg control.WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		r.noteWSMessage(msg.Type)
		r.Logger.Debug("ws recv", "type", msg.Type, "session_id", msg.SessionID)
		switch msg.Type {
		case control.SignalSessionReady:
			payloadBytes, _ := json.Marshal(msg.Payload)
			var ready control.SessionReadyPayload
			if err := json.Unmarshal(payloadBytes, &ready); err != nil {
				continue
			}
			if ready.SessionID == "" {
				ready.SessionID = msg.SessionID
			}
			go r.handleIncomingSession(ready)
		default:
			r.dispatchSignal(msg)
		}
	}
}

func (r *Runtime) Peers() ([]control.PeerInfo, error) {
	if err := r.load(); err != nil {
		return nil, err
	}
	if r.api.Token == "" {
		return nil, errors.New("not logged in; run mynet login")
	}
	return r.api.Peers()
}

// Connect starts ICE with the named peer (caller is controlling).
func (r *Runtime) Connect(peerName string) error {
	return r.ConnectWithOptions(peerName, ConnectOptions{})
}

// ConnectWithOptions starts ICE with optional relay-only mode.
func (r *Runtime) ConnectWithOptions(peerName string, opts ConnectOptions) error {
	if err := r.load(); err != nil {
		return err
	}
	if r.api.Token == "" {
		return errors.New("not logged in; run mynet login")
	}
	if opts.RelayOnly {
		r.refreshICEServers()
		if !hasTURNServer(r.serverICE) && !hasTURNServer(config.BuildICEServers(r.cfg, nil)) {
			return errors.New("relay-only requires TURN (server MYNET_TURN_* or client turn_urls); run mynet login after server TURN is configured")
		}
	}
	r.signalingMu.Lock()
	sigLoop := r.signalingRunning
	r.signalingMu.Unlock()
	if sigLoop {
		if err := r.WaitForSignaling(context.Background(), 15*time.Second); err != nil {
			return err
		}
	} else if err := r.ensureWebSocket(); err != nil {
		return err
	}
	waitCh := make(chan error, 1)
	r.mu.Lock()
	r.connectWait = waitCh
	r.peerName = peerName
	r.connectOpts = opts
	r.lastLocalCandidates = nil
	r.lastRemoteCandidates = nil
	r.mu.Unlock()

	r.cancelPeerICEAttempt(peerName)
	_, err := r.api.CreateSession(peerName)
	if err != nil {
		r.mu.Lock()
		r.connectWait = nil
		r.mu.Unlock()
		return err
	}

	select {
	case err := <-waitCh:
		r.mu.Lock()
		r.connectWait = nil
		r.mu.Unlock()
		return err
	case <-time.After(45 * time.Second):
		r.mu.Lock()
		r.connectWait = nil
		r.mu.Unlock()
		return errors.New("ICE connection timed out")
	}
}

func (r *Runtime) peerICELock(peerName string) *sync.Mutex {
	key := peerName
	if key == "" {
		key = "_"
	}
	r.peerICELockMu.Lock()
	if r.peerICEHandleMu == nil {
		r.peerICEHandleMu = make(map[string]*sync.Mutex)
	}
	m, ok := r.peerICEHandleMu[key]
	if !ok {
		m = &sync.Mutex{}
		r.peerICEHandleMu[key] = m
	}
	r.peerICELockMu.Unlock()
	return m
}

// beginIncomingICESession registers session_ready handling. Returns false if this session is already in flight.
func (r *Runtime) beginIncomingICESession(ready control.SessionReadyPayload) bool {
	r.mu.Lock()
	if r.incomingICE[ready.SessionID] {
		r.mu.Unlock()
		iceTrace("session_ready_skip", map[string]string{"session_id": ready.SessionID, "reason": "already_handling"})
		return false
	}
	if ready.PeerName != "" && r.vpnMode {
		if l, ok := r.peerLinks[ready.PeerName]; ok && l != nil && l.Conn != nil && !r.peerLinkStaleLocked(l) {
			if l.SessionID == ready.SessionID {
				r.mu.Unlock()
				iceTrace("session_ready_skip", map[string]string{"session_id": ready.SessionID, "reason": "already_transport"})
				return false
			}
			r.supersedeVPNPeerLinkLocked(ready.PeerName, l, "coordinator_new_session")
		}
	}
	if ready.PeerName != "" {
		inflight := r.peerInFlightICE[ready.PeerName]
		if inflight != "" && inflight != ready.SessionID {
			if r.incomingICE[inflight] {
				allow := ready.Role == "controlled"
				if ready.Role == "controlling" && r.peerICESupersedeAllow != nil && r.peerICESupersedeAllow[ready.PeerName] {
					allow = true
					r.peerICESupersedeAllow[ready.PeerName] = false
				}
				if !allow {
					r.mu.Unlock()
					iceTrace("session_ready_skip", map[string]string{"session_id": ready.SessionID, "reason": "stale_session_inflight"})
					return false
				}
				r.mu.Unlock()
				r.cancelPeerICEAttempt(ready.PeerName)
				r.mu.Lock()
			} else {
				r.mu.Unlock()
				iceTrace("session_ready_skip", map[string]string{"session_id": ready.SessionID, "reason": "superseded_session"})
				return false
			}
		}
	}
	r.incomingICE[ready.SessionID] = true
	if ready.PeerName != "" {
		r.peerInFlightICE[ready.PeerName] = ready.SessionID
	}
	if r.activeSession != "" && r.activeSession != ready.SessionID {
		if r.iceCancel != nil {
			r.iceCancel()
			r.iceCancel = nil
			r.iceCancelFor = ""
		}
		if r.dataConn != nil {
			_ = r.dataConn.Close()
			r.dataConn = nil
		}
		if r.iceSession != nil {
			_ = r.iceSession.Close()
			r.iceSession = nil
		}
	}
	r.activeSession = ready.SessionID
	r.peerName = ready.PeerName
	r.connState = StateConnecting
	r.mu.Unlock()
	return true
}

func (r *Runtime) handleIncomingSession(ready control.SessionReadyPayload) {
	if ready.PeerName != "" {
		r.syncPeerPresence(ready.PeerName)
	}
	peerLock := r.peerICELock(ready.PeerName)
	peerLock.Lock()
	if !r.beginIncomingICESession(ready) {
		peerLock.Unlock()
		return
	}
	peerLock.Unlock()

	iceTrace("session_ready", map[string]string{
		"session_id": ready.SessionID,
		"peer":       ready.PeerName,
		"role":       ready.Role,
	})
	controlling := ready.Role == "controlling"
	r.mu.Lock()
	opts := r.connectOpts
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.incomingICE, ready.SessionID)
		if ready.PeerName != "" && r.peerInFlightICE[ready.PeerName] == ready.SessionID {
			delete(r.peerInFlightICE, ready.PeerName)
		}
		r.mu.Unlock()
	}()
	iceCtx, endICE := r.beginPeerICEAttempt(ready.PeerName)
	defer endICE()
	conn, sess, path, err := r.runICESession(iceCtx, ready.SessionID, ready.PeerName, controlling, opts)
	if err != nil {
		r.Logger.Error("ICE session failed", "session", ready.SessionID, "controlling", controlling, "err", err)
		r.recordConnError(ready.PeerName, ready.SessionID, "ice_session", err)
		r.mu.Lock()
		if !r.vpnMode {
			r.connState = StateFailed
		}
		if !r.vpnMode {
			r.activeSession = ""
		}
		r.mu.Unlock()
		if r.vpnMode {
			r.failPeerVPN(ready.PeerName, err)
			r.mu.Lock()
			if r.activeSession == ready.SessionID {
				r.activeSession = ""
			}
			if r.connState == StateConnecting {
				r.connState = StateOnline
			}
			r.mu.Unlock()
		}
	} else if r.vpnMode && conn != nil {
		if r.vpnPeerLinkAdoptable(ready.PeerName, ready.SessionID) {
			r.storePeerLink(ready.PeerName, ready.SessionID, conn, path, sess)
		} else {
			_ = conn.Close()
			if sess != nil {
				_ = sess.Close()
			}
			iceTrace("ice_store_skipped", map[string]string{"session_id": ready.SessionID, "reason": "stale_transport"})
		}
	}
	if controlling && !r.vpnMode {
		r.mu.Lock()
		ch := r.connectWait
		r.mu.Unlock()
		if ch != nil {
			if err != nil {
				ch <- err
			} else {
				ch <- nil
			}
		}
	}
}

type signalState struct {
	mu              sync.Mutex
	iceSess         *icepkg.Session
	remoteUfrag     string
	remotePwd       string
	remoteCredReady bool
	candidates      []string
	gatherDone      bool
	peerGatherDone  bool
}

func (r *Runtime) runICESession(iceCtx context.Context, sessionID, peerName string, controlling bool, opts ConnectOptions) (net.Conn, *icepkg.Session, icepkg.PathInfo, error) {
	iceTrace("ice_begin", map[string]string{
		"session_id":  sessionID,
		"peer":        peerName,
		"controlling": boolString(controlling),
	})
	var attempt *connAttempt
	if r.vpnMode && peerName != "" && r.connTiming != nil {
		r.NoteICEAttemptStarted(peerName)
		attempt = r.connTiming.beginAttempt(peerName, sessionID)
		r.connTiming.mark(attempt, StagePeerSessionCreated)
	}
	if err := r.ensureWebSocket(); err != nil {
		if attempt != nil {
			r.connTiming.fail(attempt, err.Error())
		}
		r.recordConnError(peerName, sessionID, "signaling", err)
		return nil, nil, icepkg.PathInfo{}, err
	}
	if attempt != nil {
		r.connTiming.mark(attempt, StageSignalingReady)
	}
	if r.vpnMode {
		r.mu.Lock()
		r.lastRemoteCandidates = nil
		r.lastLocalCandidates = nil
		r.mu.Unlock()
	}
	sig := &signalState{}

	// Register signaling handler first so peer ICE messages are not dropped.
	r.setSignalHandler(sessionID, func(msg control.WSMessage) {
		switch msg.Type {
		case control.SignalICECredentials:
			payloadBytes, _ := json.Marshal(msg.Payload)
			var cred control.ICECredentialsPayload
			if json.Unmarshal(payloadBytes, &cred) == nil {
				sig.mu.Lock()
				wasReady := sig.remoteCredReady
				sig.remoteUfrag = cred.Ufrag
				sig.remotePwd = cred.Pwd
				sig.remoteCredReady = true
				sig.mu.Unlock()
				if !wasReady && attempt != nil {
					r.connTiming.mark(attempt, StageRemoteCredReceived)
				}
			}
		case control.SignalICECandidate:
			payloadBytes, _ := json.Marshal(msg.Payload)
			var cand control.ICECandidatePayload
			if json.Unmarshal(payloadBytes, &cand) == nil {
				if iceSess := sig.iceSess; iceSess != nil {
					_ = iceSess.AddRemoteCandidate(cand.Candidate)
				} else {
					sig.mu.Lock()
					sig.candidates = append(sig.candidates, cand.Candidate)
					sig.mu.Unlock()
				}
				r.mu.Lock()
				r.lastRemoteCandidates = append(r.lastRemoteCandidates, icepkg.SummarizeRemoteCandidate(cand.Candidate))
				r.mu.Unlock()
				if attempt != nil {
					r.connTiming.mark(attempt, StageRemoteCandidate)
				}
			}
		case control.SignalGatherDone:
			sig.mu.Lock()
			sig.peerGatherDone = true
			sig.mu.Unlock()
		}
	})
	defer r.clearSignalHandler(sessionID)

	sess, err := icepkg.NewSession(icepkg.SessionConfig{
		Controlling:              controlling,
		ICEServers:               r.iceServersForSession(),
		AllowTailscaleCandidates: r.cfg.ICESessionAllowTailscale(),
		ExcludeCIDRs:             r.cfg.ParsedExcludeCIDRs(),
		ExcludeInterfaces:        r.cfg.EffectiveExcludeInterfaces(),
		RelayOnly:                opts.RelayOnly,
	})
	if err != nil {
		return nil, nil, icepkg.PathInfo{}, err
	}
	sig.iceSess = sess

	sig.mu.Lock()
	pending := append([]string(nil), sig.candidates...)
	sig.mu.Unlock()
	for _, c := range pending {
		_ = sess.AddRemoteCandidate(c)
	}

	if err := sess.SetCandidateHandler(func(cand string) {
		if cand == "" {
			_ = r.sendSignal(sessionID, control.WSMessage{
				Type:    control.SignalGatherDone,
				Payload: map[string]string{"status": "complete"},
			})
			sig.mu.Lock()
			sig.gatherDone = true
			sig.mu.Unlock()
			return
		}
		_ = r.sendSignal(sessionID, control.WSMessage{
			Type:    control.SignalICECandidate,
			Payload: control.ICECandidatePayload{Candidate: cand},
		})
	}); err != nil {
		return nil, nil, icepkg.PathInfo{}, err
	}

	ufrag, pwd, err := sess.LocalCredentials()
	if err != nil {
		return nil, nil, icepkg.PathInfo{}, err
	}
	if err := r.sendSignal(sessionID, control.WSMessage{
		Type:    control.SignalICECredentials,
		Payload: control.ICECredentialsPayload{Ufrag: ufrag, Pwd: pwd},
	}); err != nil {
		return nil, nil, icepkg.PathInfo{}, err
	}

	deadline := time.Now().Add(icepkg.ConnectTimeout)
	for time.Now().Before(deadline) {
		if err := iceCtx.Err(); err != nil {
			iceTrace("ice_session_superseded", map[string]string{"session_id": sessionID})
			if attempt != nil {
				r.connTiming.fail(attempt, err.Error())
			}
			return nil, nil, icepkg.PathInfo{}, fmt.Errorf("ice session superseded: %w", err)
		}
		sig.mu.Lock()
		ready := sig.remoteCredReady
		sig.mu.Unlock()
		if ready {
			break
		}
		select {
		case <-iceCtx.Done():
			if attempt != nil {
				r.connTiming.fail(attempt, iceCtx.Err().Error())
			}
			return nil, nil, icepkg.PathInfo{}, fmt.Errorf("ice session superseded: %w", iceCtx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	sig.mu.Lock()
	if !sig.remoteCredReady {
		sig.mu.Unlock()
		if err := iceCtx.Err(); err != nil {
			if attempt != nil {
				r.connTiming.fail(attempt, err.Error())
			}
			return nil, nil, icepkg.PathInfo{}, fmt.Errorf("ice session superseded: %w", err)
		}
		iceTrace("ice_remote_credentials_timeout", map[string]string{"session_id": sessionID})
		err := errors.New("timed out waiting for peer ICE credentials")
		if attempt != nil {
			r.connTiming.fail(attempt, err.Error())
		}
		r.recordConnError(peerName, sessionID, "ice_credentials", err)
		return nil, nil, icepkg.PathInfo{}, err
	}
	remoteUfrag, remotePwd := sig.remoteUfrag, sig.remotePwd
	sig.mu.Unlock()
	iceTrace("ice_remote_credentials_ready", map[string]string{"session_id": sessionID})

	gatherStart := time.Now()
	if err := sess.Gather(); err != nil {
		if attempt != nil {
			r.connTiming.fail(attempt, err.Error())
		}
		r.recordConnError(peerName, sessionID, "ice_gather", err)
		return nil, nil, icepkg.PathInfo{}, err
	}
	if attempt != nil {
		r.connTiming.mark(attempt, StageGatherStarted)
	}
	iceTrace("ice_gather_started", map[string]string{"session_id": sessionID})
	if err := waitGatherComplete(sig, 8*time.Second); err != nil {
		iceTrace("ice_gather_timeout", map[string]string{"session_id": sessionID, "agent_state": sess.AgentState()})
		if attempt != nil {
			r.connTiming.fail(attempt, err.Error())
		}
		r.recordConnError(peerName, sessionID, "ice_gather", err)
		return nil, nil, icepkg.PathInfo{}, err
	}
	gatherDur := time.Since(gatherStart)
	if attempt != nil {
		r.connTiming.mark(attempt, StageGatherCompleted)
	}
	iceTrace("ice_gather_complete", map[string]string{"session_id": sessionID, "agent_state": sess.AgentState()})
	loc, err := sess.LocalCandidates()
	if err == nil {
		r.mu.Lock()
		r.lastLocalCandidates = loc
		counts := icepkg.CountCandidates(loc)
		r.lastGatherReport = icepkg.GatherReport{
			Duration:        gatherDur,
			AgentState:      sess.AgentState(),
			Local:           counts,
			ExcludePatterns: r.cfg.EffectiveExcludeInterfaces(),
			ExcludeCIDRs:    r.cfg.EffectiveExcludeCIDRs(),
			ICEServerURLs:   icepkg.URLsFromICEServers(r.iceServersForSession()),
		}
		r.mu.Unlock()
	}
	if opts.RelayOnly {
		hasRelay := false
		for _, c := range loc {
			if strings.Contains(c.Type, "relay") {
				hasRelay = true
				break
			}
		}
		if !hasRelay {
			return nil, nil, icepkg.PathInfo{}, errors.New("relay-only: no local TURN relay candidates gathered (check TURN reachability and credentials)")
		}
	}

	dialCtx, dialCancel := context.WithTimeout(iceCtx, icepkg.ConnectTimeout)
	r.mu.Lock()
	if r.iceCancel != nil && r.iceCancelFor != sessionID {
		r.iceCancel()
	}
	r.iceCancel = dialCancel
	r.iceCancelFor = sessionID
	r.mu.Unlock()
	defer func() {
		dialCancel()
		r.mu.Lock()
		if r.iceCancelFor == sessionID {
			r.iceCancel = nil
			r.iceCancelFor = ""
		}
		r.mu.Unlock()
	}()

	if attempt != nil {
		r.connTiming.mark(attempt, StageICEConnectStarted)
	}
	iceTrace("ice_connect_start", map[string]string{
		"session_id":  sessionID,
		"agent_state": sess.AgentState(),
		"controlling": boolString(controlling),
	})
	conn, err := sess.Connect(dialCtx, remoteUfrag, remotePwd)
	if err != nil {
		iceTrace("ice_connect_failed", map[string]string{
			"session_id":  sessionID,
			"agent_state": sess.AgentState(),
			"err":         err.Error(),
		})
		if attempt != nil {
			r.connTiming.fail(attempt, err.Error())
		}
		r.recordConnError(peerName, sessionID, "ice_connect", err)
		return nil, nil, icepkg.PathInfo{}, fmt.Errorf("ICE failed: %w", err)
	}
	if attempt != nil {
		r.connTiming.mark(attempt, StageICEConnected)
	}
	iceTrace("ice_connect_ok", map[string]string{"session_id": sessionID, "agent_state": sess.AgentState()})
	path := sess.Path()

	r.mu.Lock()
	if r.vpnMode {
		r.mu.Unlock()
		if attempt != nil {
			r.connTiming.mark(attempt, StageDataTransportReady)
			r.connTiming.succeed(attempt)
			r.NoteICEAttemptSucceeded(peerName, attempt.attempt, time.Since(attempt.start))
		}
		r.clearConnErrorForPeer(peerName)
		return conn, sess, path, nil
	}
	r.dataConn = conn
	r.iceSession = sess
	r.icePath = path
	r.connState = StateConnected
	r.peerName = peerName
	r.activeSession = sessionID
	r.mu.Unlock()

	// Controlled peer services inbound app frames (e.g. ping). Controlling peer reads in PingRTT.
	if !controlling {
		go r.readDataLoop(conn)
	}
	return conn, sess, path, nil
}

func (r *Runtime) readDataLoop(conn net.Conn) {
	for {
		frame, err := peer.ReadFrame(conn, 60*time.Second)
		if err != nil {
			return
		}
		if resp := peer.RespondPing(frame); resp != nil {
			_, _ = conn.Write(resp)
		}
	}
}

// Ping sends an application-level ping over the established ICE connection.
func (r *Runtime) Ping(peerName string) (time.Duration, error) {
	if err := r.load(); err != nil {
		return 0, err
	}
	r.mu.Lock()
	conn := r.dataConn
	state := r.connState
	if peerName != "" && r.peerName != peerName {
		r.mu.Unlock()
		return 0, fmt.Errorf("not connected to peer %q", peerName)
	}
	r.mu.Unlock()
	if state != StateConnected || conn == nil {
		return 0, errors.New("not directly connected; run mynet connect --peer <name>")
	}
	rtt, err := peer.PingRTT(conn, 5*time.Second)
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	r.lastRTT = rtt
	r.lastPingAt = time.Now()
	r.mu.Unlock()
	return rtt, nil
}

// Status snapshot for CLI output.
type Status struct {
	DeviceName string
	PeerName   string
	State      ConnState
	Transport  string
	Connection string
	TURNUsed   bool
	Local      string
	Remote     string
	LocalType  string
	RemoteType string
	ICEState   string
	RTT        time.Duration
	LastError  string
	ServerURL  string
	Signaling  string
}

func (r *Runtime) Status() (Status, error) {
	if err := r.load(); err != nil {
		return Status{}, err
	}
	r.mu.Lock()
	st := Status{
		DeviceName: r.identity.Name,
		PeerName:   r.peerName,
		State:      r.connState,
		Transport:  r.icePath.Transport,
		Connection: r.icePath.Connection,
		TURNUsed:   r.icePath.TURNUsed,
		Local:      r.icePath.LocalAddr,
		Remote:     r.icePath.RemoteAddr,
		LocalType:  r.icePath.LocalType,
		RemoteType: r.icePath.RemoteType,
		ICEState:   r.icePath.ConnectionState,
		RTT:        r.lastRTT,
		LastError:  r.lastErr,
		ServerURL:  r.cfg.ServerURL,
		Signaling:  "Disconnected",
	}
	snap := r.signalingSnapshotLocked()
	r.mu.Unlock()
	st.Signaling = snap.State
	if st.Signaling == "Disconnected" && snap.LastDisconnectReason != "" {
		st.LastError = snap.LastDisconnectReason
	} else if st.Signaling == "Connected" {
		st.LastError = ""
	}
	if st.State == StateDisconnected && r.api != nil && r.api.Token != "" {
		st.State = StateRegistered
	}
	return st, nil
}

func (r *Runtime) sendSignal(sessionID string, msg control.WSMessage) error {
	r.mu.Lock()
	ws := r.ws
	r.mu.Unlock()
	if ws == nil {
		return errors.New("signaling websocket not connected")
	}
	msg.SessionID = sessionID
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return ws.WriteMessage(websocket.TextMessage, data)
}

const maxPendingPerSession = 64

func (r *Runtime) setSignalHandler(sessionID string, fn func(control.WSMessage)) {
	r.signalMu.Lock()
	r.signalHandlers[sessionID] = fn
	queued := r.pendingSignals[sessionID]
	delete(r.pendingSignals, sessionID)
	r.signalMu.Unlock()
	for _, msg := range queued {
		fn(msg)
	}
}

func (r *Runtime) clearSignalHandler(sessionID string) {
	r.signalMu.Lock()
	delete(r.signalHandlers, sessionID)
	delete(r.pendingSignals, sessionID)
	r.signalMu.Unlock()
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func (r *Runtime) dispatchSignal(msg control.WSMessage) {
	r.signalMu.Lock()
	fn := r.signalHandlers[msg.SessionID]
	if fn == nil {
		q := r.pendingSignals[msg.SessionID]
		if len(q) < maxPendingPerSession {
			r.pendingSignals[msg.SessionID] = append(q, msg)
		}
		r.signalMu.Unlock()
		return
	}
	r.signalMu.Unlock()
	fn(msg)
}

// Down closes connections without touching system networking.
func (r *Runtime) Down() error {
	r.mu.Lock()
	if r.wsCancel != nil {
		r.wsCancel()
	}
	if r.ws != nil {
		_ = r.ws.Close()
		r.ws = nil
	}
	if r.dataConn != nil {
		_ = r.dataConn.Close()
		r.dataConn = nil
	}
	if r.iceSession != nil {
		_ = r.iceSession.Close()
		r.iceSession = nil
	}
	r.connState = StateDisconnected
	r.activeSession = ""
	r.mu.Unlock()
	return nil
}
