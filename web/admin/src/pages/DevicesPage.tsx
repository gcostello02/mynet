import { useEffect, useMemo, useState } from "react";
import { ApiError, api, type Device } from "../api/client";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { StatusBadge } from "../components/StatusBadge";

export function DevicesPage() {
  const [devices, setDevices] = useState<Device[] | null>(null);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<"all" | "active" | "revoked">("all");
  const [selected, setSelected] = useState<Device | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<Device | null>(null);
  const [revokeBusy, setRevokeBusy] = useState(false);
  const [revokeMsg, setRevokeMsg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    try {
      setDevices(await api.devices());
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to load devices");
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const filtered = useMemo(() => {
    if (!devices) return [];
    return devices.filter((d) => {
      if (filter === "active" && d.revoked) return false;
      if (filter === "revoked" && !d.revoked) return false;
      const q = query.trim().toLowerCase();
      if (!q) return true;
      return d.name.toLowerCase().includes(q) || d.device_id.toLowerCase().includes(q);
    });
  }, [devices, query, filter]);

  async function confirmRevoke() {
    if (!revokeTarget) return;
    setRevokeBusy(true);
    setRevokeMsg(null);
    try {
      await api.revokeDevice(revokeTarget.device_id);
      setRevokeMsg(`Device “${revokeTarget.name}” was revoked.`);
      setRevokeTarget(null);
      setSelected(null);
      await load();
    } catch (e) {
      setRevokeMsg(e instanceof ApiError ? e.message : "Revocation failed");
    } finally {
      setRevokeBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Devices</h1>
          <p className="text-sm text-zinc-400">Signaling status only — not WireGuard connectivity.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <input
            placeholder="Search name or ID"
            className="rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm light:border-zinc-300 light:bg-white"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <select
            className="rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm light:border-zinc-300 light:bg-white"
            value={filter}
            onChange={(e) => setFilter(e.target.value as typeof filter)}
          >
            <option value="all">All</option>
            <option value="active">Active</option>
            <option value="revoked">Revoked</option>
          </select>
        </div>
      </div>
      {error && <p className="text-red-300" role="alert">{error}</p>}
      {revokeMsg && <p className="text-sm text-emerald-300" role="status">{revokeMsg}</p>}
      {!devices ? (
        <p className="text-zinc-500">Loading devices…</p>
      ) : filtered.length === 0 ? (
        <p className="text-zinc-500">No devices match your filters.</p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-zinc-800 light:border-zinc-200">
          <table className="min-w-full text-left text-sm">
            <thead className="bg-zinc-900/80 text-zinc-400 light:bg-zinc-50">
              <tr>
                <th className="px-3 py-2 font-medium">Name</th>
                <th className="px-3 py-2 font-medium">VPN IP</th>
                <th className="px-3 py-2 font-medium">Signaling</th>
                <th className="px-3 py-2 font-medium">Status</th>
                <th className="px-3 py-2 font-medium">Enrolled</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((d) => (
                <tr
                  key={d.device_id}
                  className="cursor-pointer border-t border-zinc-800 hover:bg-zinc-900/50 light:border-zinc-200"
                  onClick={() => setSelected(d)}
                >
                  <td className="px-3 py-2 font-medium">{d.name}</td>
                  <td className="px-3 py-2 font-mono text-xs">{d.vpn_ipv4 || "—"}</td>
                  <td className="px-3 py-2">
                    <StatusBadge
                      label={d.signaling_online ? "Online" : "Offline"}
                      tone={d.signaling_online ? "ok" : "muted"}
                    />
                  </td>
                  <td className="px-3 py-2">
                    <StatusBadge label={d.revoked ? "Revoked" : "Active"} tone={d.revoked ? "bad" : "ok"} />
                  </td>
                  <td className="px-3 py-2 text-zinc-400">{new Date(d.enrolled_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {selected && (
        <aside className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4 light:border-zinc-200 light:bg-white">
          <div className="flex items-start justify-between gap-2">
            <h2 className="text-lg font-semibold">{selected.name}</h2>
            <button type="button" className="text-sm text-zinc-400" onClick={() => setSelected(null)}>
              Close
            </button>
          </div>
          <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-zinc-500">Device ID</dt>
              <dd className="font-mono text-xs break-all">{selected.device_id}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Auth epoch</dt>
              <dd>{selected.auth_epoch}</dd>
            </div>
          </dl>
          {!selected.revoked && (
            <button
              type="button"
              className="mt-4 rounded-lg bg-red-600 px-3 py-2 text-sm text-white hover:bg-red-500"
              onClick={() => setRevokeTarget(selected)}
            >
              Revoke device
            </button>
          )}
        </aside>
      )}
      <ConfirmDialog
        open={revokeTarget !== null}
        title="Revoke device?"
        description={`“${revokeTarget?.name ?? ""}” will lose access on next auth check. This cannot be undone.`}
        confirmLabel="Revoke"
        busy={revokeBusy}
        onCancel={() => setRevokeTarget(null)}
        onConfirm={() => void confirmRevoke()}
      />
    </div>
  );
}
