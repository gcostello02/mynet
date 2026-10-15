# TURN testing

## Prerequisites

Server:

```bash
export MYNET_TURN_SECRET='long-random-secret'
export MYNET_TURN_URLS='turn:YOUR_PUBLIC_IP:3478?transport=udp'
```

coturn `static-auth-secret` must match `MYNET_TURN_SECRET`. Relay ports (e.g. 49152–49252) must be reachable from the internet for off-LAN clients.

Client: `mynet login` refreshes `GET /v1/ice-servers` (ephemeral username/password per device).

## Relay-only connect

```bash
mynet connect --peer minipc --relay-only
mynet status
```

Success criteria:

- No error about missing relay candidates
- `Connection: Relayed (TURN)` and `TURN Used: true`
- At least one local candidate type contains `relay` in `mynet diagnose --peer minipc`

Failure modes:

- `relay-only: no local TURN relay candidates` — firewall, wrong secret, or TURN URL not reachable
- ICE timeout — signaling or peer offline

## Test D — blocked TURN

Point `MYNET_TURN_URLS` at an invalid host or block UDP 3478; `--relay-only` must fail with a clear error (no false connected state).

## Not verified in CI

Automated tests use local ICE pairs without real NAT or coturn. External relay validation requires manual hotspot + public TURN steps above.
