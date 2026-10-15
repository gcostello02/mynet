# Mini PC coordination server (persistent SQLite)

Use this on the host that runs `mynet-server` behind Cloudflare Tunnel (`https://mesh.example.com`).

## Layout

| Path | Purpose |
|------|---------|
| `~/mynet/env.sh` | `MYNET_SERVER_SECRET`, `MYNET_ENROLLMENT_SECRET`, TURN vars, `MYNET_DATA_PATH` |
| `~/mynet/mynet-server` | Server binary |
| `~/mynet/devices.db` | SQLite enrollments (`chmod 600`) |
| `~/mynet/start-mynet-server.sh` | Repeatable launcher |

## Start (always uses persistence)

```bash
cd ~/mynet
nohup ./start-mynet-server.sh >>server.log 2>&1 &
```

`start-mynet-server.sh` sources `env.sh` and runs:

```bash
./mynet-server --listen :8787 --data "$MYNET_DATA_PATH"
```

Default `MYNET_DATA_PATH` is `$HOME/mynet/devices.db`.

## Verify

```bash
curl -s http://127.0.0.1:8787/healthz          # ok
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8787/v1/vpn/config   # 401
grep "loaded devices from database" server.log | tail -1
```

## Re-enrollment after first SQLite enable

If the server previously ran **without** `--data`, in-memory device records were lost on restart. Existing clients keep their Ed25519 keys in `~/.mynet/identity.json`; run **`mynet login`** on each device (no `mynet init`) so the server re-authenticates and `PersistDevice` writes rows into `devices.db`.

Do not change `MYNET_ENROLLMENT_SECRET` unless you intend to invalidate old enrollment codes.
