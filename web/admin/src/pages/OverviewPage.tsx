import { ArrowRight, MonitorSmartphone, Plus, Ticket } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError, api, type Device, type Invitation } from "../api/client";
import { MembershipMap } from "../components/MembershipMap";
import { Badge, StatusDot } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Panel, PanelHeader } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { Skeleton } from "../components/ui/Skeleton";
import { useNetwork } from "../hooks/useNetwork";
import { countDevices, sortDevicesByEnrolled } from "../lib/devices";
import { relativeTime } from "../lib/time";

function Stat({ label, value, hint, tone }: { label: string; value: string; hint: string; tone?: "ok" | "bad" }) {
  return (
    <div className="px-4 py-3.5 sm:px-5">
      <div className="text-xs font-medium text-fg-muted">{label}</div>
      <div
        className={`mt-1.5 font-mono text-2xl font-medium tabular-nums leading-none tracking-tight ${
          tone === "ok" ? "text-ok" : tone === "bad" ? "text-bad" : "text-fg"
        }`}
      >
        {value}
      </div>
      <div className="mt-2 text-xs text-fg-subtle">{hint}</div>
    </div>
  );
}

export function OverviewPage() {
  const navigate = useNavigate();
  const { status, network, loaded } = useNetwork();
  const [devices, setDevices] = useState<Device[] | null>(null);
  const [invites, setInvites] = useState<Invitation[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancel = false;
    Promise.all([api.devices(), api.invitations()])
      .then(([d, i]) => {
        if (cancel) return;
        setDevices(d);
        setInvites(i);
      })
      .catch((e: unknown) => {
        if (!cancel) setError(e instanceof ApiError ? e.message : "Could not load network data.");
      });
    return () => {
      cancel = true;
    };
  }, []);

  const counts = useMemo(() => (devices ? countDevices(devices) : null), [devices]);
  const recent = useMemo(() => (devices ? sortDevicesByEnrolled(devices).slice(0, 5) : []), [devices]);
  const activeInvites = useMemo(
    () => (invites ?? []).filter((i) => !i.revoked && new Date(i.expires_at) > new Date() && i.uses < i.max_uses).length,
    [invites],
  );

  const ready = counts !== null && loaded;

  return (
    <div>
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div className="min-w-0">
          <div className="text-xs font-medium text-fg-subtle">Network</div>
          <h1 className="mt-0.5 truncate text-xl font-semibold tracking-tight">
            {network?.network_display_name || (loaded ? "Unnamed network" : " ")}
          </h1>
          {network?.public_server_url && (
            <p className="mt-1 truncate font-mono text-xs text-fg-subtle">{network.public_server_url}</p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button onClick={() => navigate("/devices")}>
            <MonitorSmartphone className="h-3.5 w-3.5" aria-hidden />
            Devices
          </Button>
          <Button variant="primary" onClick={() => navigate("/invitations?new=1")}>
            <Plus className="h-3.5 w-3.5" aria-hidden />
            Invite device
          </Button>
        </div>
      </header>

      {error && (
        <p role="alert" className="mb-4 rounded-md border border-bad/30 bg-bad-soft px-3 py-2 text-[13px] text-bad">
          {error}
        </p>
      )}

      <Panel className="mb-4">
        <dl className="grid grid-cols-2 divide-x divide-y divide-line sm:grid-cols-4 sm:divide-y-0 [&>div:nth-child(odd)]:border-l-0 [&>div:nth-child(-n+2)]:border-t-0">
          {ready && counts && status ? (
            <>
              <Stat label="Registered devices" value={String(counts.total)} hint="Enrolled with this coordinator" />
              <Stat label="Authorized" value={String(counts.active)} hint="Not revoked" />
              <Stat
                label="Signaling online"
                value={String(counts.online)}
                hint={`of ${counts.active} authorized`}
                tone={counts.online > 0 ? "ok" : undefined}
              />
              <Stat label="Revoked" value={String(counts.revoked)} hint="Access removed" tone={counts.revoked > 0 ? "bad" : undefined} />
            </>
          ) : (
            Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="space-y-3 px-5 py-4">
                <Skeleton className="h-3 w-24" />
                <Skeleton className="h-6 w-12" />
                <Skeleton className="h-3 w-28" />
              </div>
            ))
          )}
        </dl>
      </Panel>

      <div className="grid gap-4 lg:grid-cols-3">
        <Panel className="hidden overflow-hidden md:block lg:col-span-2">
          <PanelHeader title="Network membership" description="Devices enrolled with this coordinator." />
          {!devices ? (
            <div className="p-6">
              <Skeleton className="h-64 w-full" />
            </div>
          ) : devices.length === 0 ? (
            <EmptyState
              icon={MonitorSmartphone}
              title="No devices yet"
              description="Create an invitation, then run mynet join on the device you want to add."
              action={
                <Button variant="primary" onClick={() => navigate("/invitations?new=1")}>
                  <Plus className="h-3.5 w-3.5" /> Invite device
                </Button>
              }
            />
          ) : (
            <MembershipMap devices={devices} />
          )}
        </Panel>

        <div className="flex flex-col gap-4 md:col-span-1 lg:col-span-1">
          <Panel>
            <PanelHeader
              title="Recently enrolled"
              action={
                <Link to="/devices" className="inline-flex items-center gap-1 text-xs font-medium text-accent hover:underline">
                  All devices <ArrowRight className="h-3 w-3" aria-hidden />
                </Link>
              }
            />
            {!devices ? (
              <div className="space-y-3 p-4">
                <Skeleton className="h-8" />
                <Skeleton className="h-8" />
                <Skeleton className="h-8" />
              </div>
            ) : recent.length === 0 ? (
              <p className="px-4 py-6 text-[13px] text-fg-muted">Nothing enrolled yet.</p>
            ) : (
              <ul className="divide-y divide-line">
                {recent.map((d) => (
                  <li key={d.device_id}>
                    <Link
                      to={`/devices?device=${encodeURIComponent(d.device_id)}`}
                      className="flex items-center justify-between gap-3 px-4 py-2.5 transition-colors hover:bg-hover/60"
                    >
                      <span className="min-w-0">
                        <span className={`block truncate text-[13px] font-medium ${d.revoked ? "text-fg-subtle line-through" : ""}`}>
                          {d.name}
                        </span>
                        <span className="block text-xs text-fg-subtle" title={d.enrolled_at}>
                          {relativeTime(d.enrolled_at)}
                        </span>
                      </span>
                      {d.revoked ? (
                        <Badge tone="bad">Revoked</Badge>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 text-xs text-fg-muted">
                          <StatusDot tone={d.signaling_online ? "ok" : "neutral"} hollow={!d.signaling_online} />
                          {d.signaling_online ? "Online" : "Offline"}
                        </span>
                      )}
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Panel>

          <Panel>
            <PanelHeader title="Enrollment" />
            <dl className="divide-y divide-line text-[13px]">
              <div className="flex items-center justify-between gap-3 px-4 py-2.5">
                <dt className="text-fg-muted">Mode</dt>
                <dd>
                  {status ? (
                    status.legacy_enrollment ? (
                      <Badge tone="warn">Legacy shared secret</Badge>
                    ) : (
                      <Badge tone="ok">Invitation only</Badge>
                    )
                  ) : (
                    <Skeleton className="h-4 w-24" />
                  )}
                </dd>
              </div>
              <div className="flex items-center justify-between gap-3 px-4 py-2.5">
                <dt className="text-fg-muted">Open invitations</dt>
                <dd className="flex items-center gap-2">
                  <span className="font-mono tabular-nums">{invites ? activeInvites : "—"}</span>
                  <Link to="/invitations" className="text-accent hover:underline" aria-label="Manage invitations">
                    <Ticket className="h-3.5 w-3.5" />
                  </Link>
                </dd>
              </div>
            </dl>
          </Panel>
        </div>
      </div>
    </div>
  );
}
