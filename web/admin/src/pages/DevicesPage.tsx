import { Ban, Copy, Eye, MonitorSmartphone, MoreHorizontal, Plus } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { ApiError, api, type Device } from "../api/client";
import { DeviceIcon } from "../components/DeviceIcon";
import { Badge, StatusDot } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Panel } from "../components/ui/Card";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { CopyField } from "../components/ui/Copy";
import { Drawer } from "../components/ui/Drawer";
import { EmptyState } from "../components/ui/EmptyState";
import { SearchInput } from "../components/ui/Input";
import { Menu } from "../components/ui/Menu";
import { PageHeader } from "../components/ui/PageHeader";
import { Segmented } from "../components/ui/Segmented";
import { RowsSkeleton } from "../components/ui/Skeleton";
import { useNetwork } from "../hooks/useNetwork";
import { useToast } from "../hooks/useToast";
import { countDevices, signalingLabel, sortDevicesByEnrolled } from "../lib/devices";
import { formatDate, formatDateTime, relativeTime } from "../lib/time";

type Filter = "all" | "active" | "revoked";

function SignalingCell({ d }: { d: Device }) {
  const s = signalingLabel(d);
  return (
    <span className={`inline-flex items-center gap-1.5 text-[13px] ${d.revoked ? "text-fg-subtle" : "text-fg"}`}>
      <StatusDot tone={s.tone} hollow={s.hollow} />
      {s.text}
    </span>
  );
}

function AuthBadge({ d }: { d: Device }) {
  return d.revoked ? <Badge tone="bad">Revoked</Badge> : <Badge tone="ok">Authorized</Badge>;
}

export function DevicesPage() {
  const navigate = useNavigate();
  const { push } = useToast();
  const { refresh } = useNetwork();
  const [params, setParams] = useSearchParams();
  const [devices, setDevices] = useState<Device[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [revokeTarget, setRevokeTarget] = useState<Device | null>(null);
  const [revoking, setRevoking] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    try {
      setDevices(await api.devices());
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not load devices.");
    }
  }, []);

  useEffect(() => {
    void load();
    const t = window.setInterval(() => {
      if (document.visibilityState === "visible") void load();
    }, 15000);
    return () => window.clearInterval(t);
  }, [load]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      if (e.key === "/" && !/^(INPUT|TEXTAREA|SELECT)$/.test(el?.tagName ?? "") && !el?.isContentEditable) {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const counts = useMemo(() => countDevices(devices ?? []), [devices]);
  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return sortDevicesByEnrolled(devices ?? []).filter((d) => {
      if (filter === "active" && d.revoked) return false;
      if (filter === "revoked" && !d.revoked) return false;
      if (!q) return true;
      return d.name.toLowerCase().includes(q) || d.vpn_ipv4.includes(q) || d.device_id.toLowerCase().includes(q);
    });
  }, [devices, query, filter]);

  const selectedId = params.get("device");
  const selected = devices?.find((d) => d.device_id === selectedId) ?? null;
  const select = (id: string | null) => {
    const next = new URLSearchParams(params);
    if (id) next.set("device", id);
    else next.delete("device");
    setParams(next, { replace: true });
  };

  async function revoke() {
    if (!revokeTarget) return;
    setRevoking(true);
    try {
      await api.revokeDevice(revokeTarget.device_id);
      push(`${revokeTarget.name} was revoked.`, "success");
      setRevokeTarget(null);
      await load();
      void refresh();
    } catch (e) {
      push(e instanceof ApiError ? e.message : "Revocation failed.", "error");
    } finally {
      setRevoking(false);
    }
  }

  const copyId = (d: Device) => {
    void navigator.clipboard
      .writeText(d.device_id)
      .then(() => push("Device ID copied.", "success"))
      .catch(() => push("Could not access the clipboard.", "error"));
  };

  const actions = (d: Device) => [
    { label: "View details", icon: <Eye className="h-4 w-4" />, onSelect: () => select(d.device_id) },
    { label: "Copy device ID", icon: <Copy className="h-4 w-4" />, onSelect: () => copyId(d) },
    ...(d.revoked
      ? []
      : [
          { type: "separator" as const },
          { label: "Revoke…", icon: <Ban className="h-4 w-4" />, danger: true, onSelect: () => setRevokeTarget(d) },
        ]),
  ];

  const loading = devices === null && !error;

  return (
    <div>
      <PageHeader
        title="Devices"
        description="Every device enrolled with this coordinator. Signaling status shows whether a device is connected to the coordinator, not whether a VPN tunnel is up."
        action={
          <Button variant="primary" onClick={() => navigate("/invitations?new=1")}>
            <Plus className="h-3.5 w-3.5" aria-hidden /> Invite device
          </Button>
        }
      />

      <div className="mb-3 flex flex-wrap items-center gap-2">
        <SearchInput
          ref={searchRef}
          label="Search devices"
          placeholder="Search name, address, ID"
          value={query}
          onChange={setQuery}
          className="w-full sm:w-72"
        />
        <Segmented
          label="Filter by authorization"
          value={filter}
          onChange={setFilter}
          options={[
            { value: "all", label: "All", count: counts.total },
            { value: "active", label: "Authorized", count: counts.active },
            { value: "revoked", label: "Revoked", count: counts.revoked },
          ]}
        />
      </div>

      {error && (
        <p role="alert" className="mb-3 rounded-md border border-bad/30 bg-bad-soft px-3 py-2 text-[13px] text-bad">
          {error}
        </p>
      )}

      <Panel className="overflow-hidden">
        {loading ? (
          <RowsSkeleton rows={5} />
        ) : rows.length === 0 ? (
          devices && devices.length === 0 ? (
            <EmptyState
              icon={MonitorSmartphone}
              title="No devices enrolled"
              description="Create an invitation, then run mynet join on a device to enroll it."
              action={
                <Button variant="primary" onClick={() => navigate("/invitations?new=1")}>
                  <Plus className="h-3.5 w-3.5" /> Invite device
                </Button>
              }
            />
          ) : (
            <EmptyState
              icon={MonitorSmartphone}
              title="No matching devices"
              description="Nothing matches your search and filter."
              action={
                <Button
                  onClick={() => {
                    setQuery("");
                    setFilter("all");
                  }}
                >
                  Clear filters
                </Button>
              }
            />
          )
        ) : (
          <>
            {/* desktop table */}
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full text-left text-[13px]">
                <thead>
                  <tr className="border-b border-line text-xs text-fg-muted">
                    <th scope="col" className="px-4 py-2.5 font-medium">Device</th>
                    <th scope="col" className="px-4 py-2.5 font-medium">VPN address</th>
                    <th scope="col" className="px-4 py-2.5 font-medium">Signaling</th>
                    <th scope="col" className="px-4 py-2.5 font-medium">Enrolled</th>
                    <th scope="col" className="px-4 py-2.5 font-medium">Authorization</th>
                    <th scope="col" className="w-12 px-2 py-2.5"><span className="sr-only">Actions</span></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {rows.map((d) => (
                    <tr
                      key={d.device_id}
                      className={`group transition-colors hover:bg-hover/60 ${selectedId === d.device_id ? "bg-hover/60" : ""}`}
                    >
                      <td className="px-4 py-2">
                        <button
                          type="button"
                          onClick={() => select(d.device_id)}
                          className="flex items-center gap-3 rounded text-left"
                          aria-label={`View ${d.name}`}
                        >
                          <DeviceIcon name={d.name} muted={d.revoked} />
                          <span className={`font-medium ${d.revoked ? "text-fg-subtle line-through" : ""}`}>{d.name}</span>
                        </button>
                      </td>
                      <td className="px-4 py-2 font-mono text-xs tabular-nums text-fg-muted">{d.vpn_ipv4 || "—"}</td>
                      <td className="px-4 py-2"><SignalingCell d={d} /></td>
                      <td className="px-4 py-2 text-fg-muted" title={formatDateTime(d.enrolled_at)}>
                        {formatDate(d.enrolled_at)}
                      </td>
                      <td className="px-4 py-2"><AuthBadge d={d} /></td>
                      <td className="px-2 py-2 text-right">
                        <Menu
                          label={`Actions for ${d.name}`}
                          trigger={<MoreHorizontal className="h-4 w-4" aria-hidden />}
                          triggerClassName="inline-flex h-7 w-7 items-center justify-center rounded-md text-fg-muted hover:bg-hover hover:text-fg"
                          items={actions(d)}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* mobile cards */}
            <ul className="divide-y divide-line md:hidden">
              {rows.map((d) => (
                <li key={d.device_id} className="flex items-center gap-3 px-4 py-3">
                  <DeviceIcon name={d.name} muted={d.revoked} />
                  <button type="button" onClick={() => select(d.device_id)} className="min-w-0 flex-1 text-left">
                    <span className={`block truncate text-[13px] font-medium ${d.revoked ? "text-fg-subtle line-through" : ""}`}>
                      {d.name}
                    </span>
                    <span className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-fg-muted">
                      <span className="font-mono tabular-nums">{d.vpn_ipv4 || "—"}</span>
                      <SignalingCell d={d} />
                    </span>
                  </button>
                  <Menu
                    label={`Actions for ${d.name}`}
                    trigger={<MoreHorizontal className="h-4 w-4" aria-hidden />}
                    triggerClassName="inline-flex h-8 w-8 items-center justify-center rounded-md text-fg-muted hover:bg-hover hover:text-fg"
                    items={actions(d)}
                  />
                </li>
              ))}
            </ul>
          </>
        )}
      </Panel>

      <Drawer
        open={!!selected}
        onClose={() => select(null)}
        eyebrow="Device"
        title={selected?.name ?? ""}
        footer={
          selected && !selected.revoked ? (
            <Button variant="danger-soft" onClick={() => setRevokeTarget(selected)}>
              <Ban className="h-3.5 w-3.5" aria-hidden /> Revoke device
            </Button>
          ) : undefined
        }
      >
        {selected && (
          <div className="space-y-5">
            <div className="flex flex-wrap items-center gap-2">
              <AuthBadge d={selected} />
              <SignalingCell d={selected} />
            </div>
            <CopyField label="Device ID" value={selected.device_id} />
            <CopyField label="VPN address" value={selected.vpn_ipv4} />
            <dl className="divide-y divide-line rounded-md border border-line text-[13px]">
              <div className="flex justify-between gap-3 px-3 py-2">
                <dt className="text-fg-muted">Enrolled</dt>
                <dd title={formatDateTime(selected.enrolled_at)}>
                  {formatDateTime(selected.enrolled_at)}{" "}
                  <span className="text-fg-subtle">({relativeTime(selected.enrolled_at)})</span>
                </dd>
              </div>
              <div className="flex justify-between gap-3 px-3 py-2">
                <dt className="text-fg-muted">Auth epoch</dt>
                <dd className="font-mono tabular-nums">{selected.auth_epoch}</dd>
              </div>
            </dl>
            <p className="text-xs leading-relaxed text-fg-subtle">
              “Signaling” reflects this device’s connection to the coordinator. It does not indicate whether a
              WireGuard tunnel to another device is established.
            </p>
            {selected.revoked && (
              <p className="rounded-md border border-bad/30 bg-bad-soft px-3 py-2 text-[13px] text-bad">
                This device is revoked. It can no longer authenticate or receive peer updates.
              </p>
            )}
          </div>
        )}
      </Drawer>

      <ConfirmDialog
        open={!!revokeTarget}
        title={`Revoke ${revokeTarget?.name ?? "device"}?`}
        description="The device will immediately lose access to this network. This cannot be undone; it must be re-enrolled with a new invitation."
        confirmLabel="Revoke device"
        requireText={revokeTarget?.name}
        busy={revoking}
        onConfirm={() => void revoke()}
        onCancel={() => setRevokeTarget(null)}
      />
    </div>
  );
}
