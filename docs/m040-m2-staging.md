# M2 isolated staging acceptance

1. Build `mynet-server` and `mynet` from `feature/v040-m2-secure-join`.
2. Export fresh `MYNET_SERVER_SECRET`, `MYNET_DATA_PATH=/tmp/m2-coord.db`.
3. Run server on `127.0.0.1:18787`; complete admin setup (localhost).
4. `mynet admin login --server https://...` (use TLS reverse proxy or local tunnel for HTTPS in real staging; tests may use HTTP with test flag only).
5. Create invitation; run `mynet join` on three temp `MYNET_STATE_DIR` paths.
6. Verify unique VPN IPs; `mynet up` or supervised service on each host.
7. Revoke one device; confirm peer drops from others within reconcile interval.

**Do not** use `mesh.example.com` or production state.
