import { useCallback, useEffect, useMemo, useState } from "react";
import { MonitorSmartphone, Search } from "lucide-react";
import { ApiError, api, type Device } from "../api/client";
import { PageHeader } from "../components/ui/PageHeader";
import { Card } from "../components/ui/Card";
import { Badge } from "../components/ui/Badge";
import { EmptyState } from "../components/ui/EmptyState";
import { TableSkeleton } from "../components/ui/Skeleton";
import { Drawer } from "../components/ui/Drawer";
import { Modal } from "../components/ui/Modal";
import { Button } from "../components/ui/Button";
import { formatEnrolled } from "../lib/devices";
import { useToast } from "../hooks/useToast";

function signalingBadge(d: Device) {
  if (d.revoked) return <Badge tone="danger">Revoked</Badge>;
  return (
    <Badge tone={d.signaling_online ? "success" : "neutral"}>
      {d.signaling_online ? "Signaling online" : "Signaling offline"}
    </Badge>
  );
}

export function DevicesPage() {
  const { push } = useToast();
  const [devices, setDevices] = useState<Device[] | null>(null);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<"all" | "active" | "revoked">("all");
  const [selected, setSelected] = useState<Device | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<Device | null>(null);
  const [revokeBusy, setRevokeBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setDevices(await api.devices());
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to load devices");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

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
    try {
      await api.revokeDevice(revokeTarget.device_id);
      push(`Device “${revokeTarget.name}” revoked.`);
      setRevokeTarget(null);
      setSelected(null);
      await load();
    } catch (e) {
      push(e instanceof ApiError ? e.message : "Revocation failed", "error");
    } finally {
      setRevokeBusy(false);
    }
  }

  return (
    <div>
      <PageHeader
        title="Devices"
        description="Inventory from the coordinator. Signaling status is not WireGuard connectivity."
      />

      <Card className="mb-4" padding>
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-fg-subtle)]" aria-hidden />
            <input
              type="search"
              placeholder="Search by name or device ID"
              aria-label="Search devices"
              className="w-full rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] py-2 pl-9 pr-3 text-sm"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <select
            className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm sm:w-44"
            value={filter}
            onChange={(e) => setFilter(e.target.value as typeof filter)}
            aria-label="Filter devices"
          >
            <option value="all">All devices</option>
            <option value="active">Active only</option>
            <option value="revoked">Revoked</option>
          </select>
        </div>
      </Card>

      {error && (
        <p className="mb-4 rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300" role="alert">
          {error}
        </p>
      )}

      <Card padding={false}>
        {!devices ? (
          <div className="p-4">
            <TableSkeleton />
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={MonitorSmartphone}
            title="No devices found"
            description={
              devices.length === 0
                ? "Enroll a device with an invitation to see it here."
                : "Try a different search or filter."
            }
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-left text-sm">
              <thead className="border-b border-[var(--color-border)] bg-[var(--color-surface-overlay)]/50 text-xs uppercase tracking-wide text-[var(--color-fg-subtle)]">
                <tr>
                  <th className="px-4 py-3 font-medium">Name</th>
                  <th className="px-4 py-3 font-medium">VPN IP</th>
                  <th className="px-4 py-3 font-medium">Signaling</th>
                  <th className="px-4 py-3 font-medium">Enrolled</th>
                  <th className="hidden px-4 py-3 font-medium sm:table-cell">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {filtered.map((d) => (
                  <tr
                    key={d.device_id}
                    className="cursor-pointer transition-colors hover:bg-[var(--color-surface-overlay)]/40"
                    onClick={() => setSelected(d)}
                  >
                    <td className="px-4 py-3 font-medium">{d.name}</td>
                    <td className="px-4 py-3 font-mono text-xs text-[var(--color-fg-muted)]">
                      {d.vpn_ipv4 || "—"}
                    </td>
                    <td className="px-4 py-3">{signalingBadge(d)}</td>
                    <td className="px-4 py-3 text-[var(--color-fg-muted)]">{formatEnrolled(d.enrolled_at)}</td>
                    <td className="hidden px-4 py-3 sm:table-cell">
                      {d.revoked ? <Badge tone="danger">Revoked</Badge> : <Badge tone="info">Active</Badge>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <Drawer open={selected !== null} onClose={() => setSelected(null)} title={selected?.name ?? "Device"}>
        {selected && (
          <div className="space-y-4 text-sm">
            <dl className="space-y-3">
              <div>
                <dt className="text-xs text-[var(--color-fg-subtle)]">Device ID</dt>
                <dd className="mt-1 break-all font-mono text-xs">{selected.device_id}</dd>
              </div>
              <div>
                <dt className="text-xs text-[var(--color-fg-subtle)]">VPN IPv4</dt>
                <dd className="mt-1 font-mono">{selected.vpn_ipv4 || "Not assigned"}</dd>
              </div>
              <div>
                <dt className="text-xs text-[var(--color-fg-subtle)]">Signaling</dt>
                <dd className="mt-1">{signalingBadge(selected)}</dd>
              </div>
              <div>
                <dt className="text-xs text-[var(--color-fg-subtle)]">Enrolled</dt>
                <dd className="mt-1">{formatEnrolled(selected.enrolled_at)}</dd>
              </div>
              <div>
                <dt className="text-xs text-[var(--color-fg-subtle)]">Auth epoch</dt>
                <dd className="mt-1 tabular-nums">{selected.auth_epoch}</dd>
              </div>
            </dl>
            <p className="text-xs text-[var(--color-fg-subtle)]">
              Signaling online means the device has an active WebSocket to the coordinator, not that a WireGuard tunnel is up.
            </p>
            {!selected.revoked && (
              <Button variant="danger" className="w-full" onClick={() => setRevokeTarget(selected)}>
                Revoke device
              </Button>
            )}
          </div>
        )}
      </Drawer>

      <Modal
        open={revokeTarget !== null}
        onClose={() => !revokeBusy && setRevokeTarget(null)}
        title="Revoke device?"
        description={`“${revokeTarget?.name ?? ""}” will lose access on the next authentication check. This cannot be undone.`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setRevokeTarget(null)} disabled={revokeBusy}>
              Cancel
            </Button>
            <Button variant="danger" onClick={() => void confirmRevoke()} disabled={revokeBusy}>
              {revokeBusy ? "Revoking…" : "Revoke device"}
            </Button>
          </>
        }
      >
        <p className="text-sm text-[var(--color-fg-muted)]">
          WireGuard peers may remain until clients reconcile; revocation is enforced at enrollment and auth.
        </p>
      </Modal>
    </div>
  );
}
