import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Activity, MonitorSmartphone, Shield, Ticket, Server } from "lucide-react";
import { ApiError, api, type AdminStatus, type Device, type NetworkPublic } from "../api/client";
import { PageHeader } from "../components/ui/PageHeader";
import { Card, CardHeader } from "../components/ui/Card";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Skeleton } from "../components/ui/Skeleton";
import { formatEnrolled, sortDevicesByEnrolled } from "../lib/devices";

export function OverviewPage() {
  const navigate = useNavigate();
  const [status, setStatus] = useState<AdminStatus | null>(null);
  const [network, setNetwork] = useState<NetworkPublic | null>(null);
  const [devices, setDevices] = useState<Device[] | null>(null);
  const [healthOk, setHealthOk] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [st, hp, devs] = await Promise.all([
          api.status(),
          api.health().then((r) => r.ok),
          api.devices(),
        ]);
        if (cancelled) return;
        setStatus(st);
        setHealthOk(hp);
        setDevices(devs);
        try {
          const np = await api.networkPublic();
          if (!cancelled) setNetwork(np);
        } catch {
          /* optional */
        }
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof ApiError ? e.message : "Failed to load overview");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <p className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300" role="alert">
        {error}
      </p>
    );
  }

  const recent = devices ? sortDevicesByEnrolled(devices).slice(0, 5) : [];
  const networkName = network?.network_display_name ?? "Your network";

  return (
    <div>
      <PageHeader
        title={networkName}
        description="Coordinator overview from live API data."
        action={
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" onClick={() => navigate("/invitations")}>
              <Ticket className="h-4 w-4" />
              Invite device
            </Button>
            <Button variant="secondary" onClick={() => navigate("/devices")}>
              <MonitorSmartphone className="h-4 w-4" />
              All devices
            </Button>
          </div>
        }
      />

      {!status ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <MetricCard
            icon={Server}
            label="Coordinator"
            value={healthOk ? "Healthy" : "Unreachable"}
            hint="/healthz on this host"
            tone={healthOk ? "success" : "danger"}
          />
          <MetricCard
            icon={MonitorSmartphone}
            label="Active devices"
            value={String(status.active_device_count)}
            hint={`${status.device_count} enrolled total`}
          />
          <MetricCard
            icon={Activity}
            label="Signaling online"
            value={String(status.signaling_online_count)}
            hint="WebSocket presence, not VPN tunnels"
          />
          <MetricCard
            icon={Shield}
            label="Revoked"
            value={String(status.revoked_device_count)}
            hint={status.legacy_enrollment ? "Legacy enrollment on" : "Invitation enrollment"}
          />
        </div>
      )}

      <div className="mt-8 grid gap-6 lg:grid-cols-5">
        <Card className="lg:col-span-3" padding>
          <CardHeader
            title="Recent devices"
            description="Sorted by enrollment time (newest first)."
            action={
              <Link to="/devices" className="text-xs font-medium text-[var(--color-accent)] hover:underline">
                View all
              </Link>
            }
          />
          {!devices ? (
            <div className="space-y-2">
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
            </div>
          ) : recent.length === 0 ? (
            <p className="text-sm text-[var(--color-fg-muted)]">No enrolled devices yet.</p>
          ) : (
            <ul className="divide-y divide-[var(--color-border)]">
              {recent.map((d) => (
                <li key={d.device_id} className="flex items-center justify-between gap-2 py-3 first:pt-0">
                  <div>
                    <p className="text-sm font-medium">{d.name}</p>
                    <p className="text-xs text-[var(--color-fg-subtle)]">
                      Enrolled {formatEnrolled(d.enrolled_at)}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    {d.revoked ? (
                      <Badge tone="danger">Revoked</Badge>
                    ) : (
                      <Badge tone={d.signaling_online ? "success" : "neutral"}>
                        {d.signaling_online ? "Signaling online" : "Signaling offline"}
                      </Badge>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card className="lg:col-span-2" padding>
          <CardHeader title="Security" description="Enrollment mode from coordinator status." />
          {status ? (
            <dl className="space-y-3 text-sm">
              <div>
                <dt className="text-[var(--color-fg-subtle)]">Enrollment</dt>
                <dd className="mt-0.5 font-medium">
                  {status.legacy_enrollment ? "Legacy shared secret" : "Invitation-based"}
                </dd>
              </div>
              <div>
                <dt className="text-[var(--color-fg-subtle)]">Server</dt>
                <dd className="mt-0.5 font-mono text-xs">{status.server_version}</dd>
              </div>
              <div>
                <dt className="text-[var(--color-fg-subtle)]">Fingerprint</dt>
                <dd className="mt-0.5 break-all font-mono text-xs text-[var(--color-fg-muted)]">
                  {status.coordinator_fingerprint.slice(0, 24)}…
                </dd>
              </div>
            </dl>
          ) : (
            <Skeleton className="h-32" />
          )}
        </Card>
      </div>
    </div>
  );
}

function MetricCard({
  icon: Icon,
  label,
  value,
  hint,
  tone,
}: {
  icon: typeof Server;
  label: string;
  value: string;
  hint: string;
  tone?: "success" | "danger";
}) {
  return (
    <Card>
      <div className="flex items-start justify-between gap-2">
        <div className="flex h-9 w-9 items-center justify-center rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-overlay)]">
          <Icon className="h-4 w-4 text-[var(--color-fg-muted)]" aria-hidden />
        </div>
        {tone === "success" && <Badge tone="success">OK</Badge>}
        {tone === "danger" && <Badge tone="danger">Issue</Badge>}
      </div>
      <p className="mt-3 text-xs font-medium uppercase tracking-wide text-[var(--color-fg-subtle)]">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
      <p className="mt-1 text-xs text-[var(--color-fg-muted)]">{hint}</p>
    </Card>
  );
}
