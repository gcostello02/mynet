# MyNet daemon profiling (development only)

Profiling is **opt-in** and **off by default**. It does not change the WireGuard/ICE data path; it only exposes Go `runtime/pprof` and `net/http/pprof` on a **Unix domain socket** in your state directory (mode `0600`, same ownership rules as `mynetd.sock`).

Never bind profiling to a public TCP address.

## Enable profiling

Restart the daemon with profiling enabled (requires a deliberate restart; not enabled on running daemons without restart):

```bash
# macOS / Linux — state dir from mynet preflight (usually ~/.mynet)
export MYNET_ENABLE_PPROF=1
# optional: collect mutex / block profiles (small runtime overhead)
export MYNET_PPROF_MUTEX=1
export MYNET_PPROF_BLOCK=1

# Stop existing daemon, then start with profiling (example)
mynet daemon stop
sudo -E env MYNET_ENABLE_PPROF=1 MYNET_PPROF_MUTEX=1 mynet up   # or run `_daemon` with same env
```

Check status:

```bash
mynet profile status
```

Sockets (when enabled):

| Socket | Purpose |
|--------|---------|
| `$STATE/mynetd.sock` | VPN control IPC (unchanged) |
| `$STATE/pprof.sock` | `net/http/pprof` over Unix HTTP |

Snapshot files written via IPC land in `$STATE/profiles/` (mode `0700`).

## Capture a 30s CPU profile during iperf3

On the **machine running mynetd** (e.g. mini PC receiving traffic):

**Terminal 1 — start iperf3 server on the VPN IP**

```bash
iperf3 -s -B 172.31.240.2
```

**Terminal 2 — CPU profile (30s) while load runs**

```bash
# Option A: CLI via daemon IPC (blocks ~30s)
iperf3 -c 172.31.240.2 -t 35 -P 4 &   # MacBook → mini
mynet profile cpu --seconds 30 -o /tmp/mynet-cpu.prof

# Option B: curl over Unix socket (same host as daemon)
curl --unix-socket "$HOME/.mynet/pprof.sock" \
  "http://localhost/debug/pprof/profile?seconds=30" \
  -o /tmp/mynet-cpu.prof

# Option C: go tool directly
go tool pprof -proto \
  "http+unix://$HOME/.mynet/pprof.sock/debug/pprof/profile?seconds=30"
```

Repeat on **MacBook** if you need the send-side profile (`iperf3 -R` or reverse direction).

### Other profiles

```bash
mynet profile snapshot --kind heap -o /tmp/heap.prof
mynet profile snapshot --kind allocs -o /tmp/allocs.prof
mynet profile snapshot --kind goroutine -o /tmp/goroutine.prof
mynet profile snapshot --kind mutex -o /tmp/mutex.prof    # needs MYNET_PPROF_MUTEX=1
mynet profile snapshot --kind block -o /tmp/block.prof    # needs MYNET_PPROF_BLOCK=1

# Or HTTP fetch:
mynet profile fetch --path /debug/pprof/heap -o /tmp/heap.prof
```

## Analyze with `go tool pprof`

```bash
go tool pprof -http=:0 /tmp/mynet-cpu.prof
# or text top:
go tool pprof -top /tmp/mynet-cpu.prof
go tool pprof -top -cum /tmp/mynet-cpu.prof
```

What to look for (MyNet userspace VPN stack):

| Area | Typical symbols / packages |
|------|----------------------------|
| WireGuard crypto | `golang.zx2c4.com/wireguard/device`, `chacha20poly1305`, `curve25519` |
| ICE / UDP | `github.com/pion/ice`, `github.com/pion/transport` |
| TUN I/O | `golang.zx2c4.com/wireguard/tun`, `Read`/`Write` syscalls |
| Copies | `ICEBind.readLoop` (`append` per datagram), channel send to wireguard-go |
| Locking | `sync.(*Mutex).Lock`, `RWMutex` in bind/engine if contended |
| GC | `runtime.gc`, `mallocgc` — high `alloc_space` in `allocs` profile |

Compare **flat** vs **cum** CPU; a single hot goroutine at ~70–80% of one core often shows up as one stack rooted in `device.(*Device).RoutineEncryption` / `RoutineDecryption` or TUN read loops.

## Code-level suspects (inspection, not yet optimized)

From the current architecture (`d341175` baseline):

1. **Userspace WireGuard** — all encrypt/decrypt runs in-process (not kernel WG); at ~150–260 Mbps this commonly saturates **one core** before the link.
2. **`ICEBind.readLoop`** — allocates a new slice per inbound datagram (`append([]byte(nil), buf[:n]...)`) before handing to wireguard-go.
3. **Single peer read loop** — one goroutine per peer reads ICE and feeds WG; no batching (`BatchSize() == 1`).
4. **Pion ICE** — extra userspace UDP encapsulation on top of kernel UDP.
5. **Asymmetric Mbps (Mac→mini ~150 vs mini→Mac ~264)** — investigate direction-specific stacks in profiles (encrypt vs decrypt, TUN read vs write paths).

Use profiles to confirm which of these dominates on **your** hardware before changing the data path.

## Security

- Profiling endpoints reveal goroutine stacks and memory layout, not WireGuard private keys or plaintext payloads.
- Do not enable `MYNET_ENABLE_PPROF` on production hosts exposed to untrusted local users.
- Coordination server (`mynet-server`) is unchanged; profiling is **daemon-only**.
