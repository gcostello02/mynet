# Developing over Tailscale

MyNet V0.1 uses ICE host candidates for the data plane. On a LAN where Tailscale is also running, physical `10.x` paths are preferred by default: host candidates on `100.64.0.0/10` are filtered out so ICE does not pick tailnet addresses when both sides are on the same LAN.

When you are **off the home LAN** but both machines are on the same tailnet, use the coordination server over a Tailscale IP and allow tailnet ICE candidates.

## Quick setup (Mac off-LAN, mini PC on tailnet)

1. On the mini PC, keep the server and a signaling client running:

   ```bash
   cd ~/mynet && source env.sh && ./mynet-server --listen :8787
   # another session:
   cd ~/mynet && source env.sh && ./mynet login --stay
   ```

2. On your Mac, point at the mini PC’s **Tailscale IP** (not `10.0.0.50`):

   ```bash
   mynet config tailnet --server http://<minipc-tailscale-ip>:8787
   mynet login
   mynet connect --peer minipc
   ```

   Or set `~/.mynet/config.json` manually:

   ```json
   {
     "server_url": "http://100.x.x.x:8787",
     "allow_tailscale_candidates": true,
     "stun_urls": ["stun:stun.l.google.com:19302"]
   }
   ```

   If `server_url` uses a `100.x` address, tailnet ICE candidates are enabled automatically even without `allow_tailscale_candidates`.

3. `mynet status` should show transport `host` with local/remote `100.x` addresses when the path is direct over Tailscale.

## TURN (optional, not required on tailnet)

For networks that block UDP or when neither LAN nor tailnet host paths work, add TURN URLs to config:

```json
{
  "turn_urls": ["turn:your-turn.example.com:3478?transport=udp"]
}
```

Credentials belong in the URL query string per Pion’s format (`?username=...&password=...`). A future version may add a dedicated TURN credential flow.

## Replacing Tailscale (roadmap)

Today: control plane + P2P app UDP over ICE. Next steps: TURN for arbitrary NAT, then WireGuard/TUN and stable MyNet IPs so the tailnet is no longer required for connectivity.
