import { useEffect, useState, type ReactNode } from "react";
import { ApiError, api, type AdminStatus, type NetworkPublic } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";

export function OverviewPage() {
  const [status, setStatus] = useState<AdminStatus | null>(null);
  const [network, setNetwork] = useState<NetworkPublic | null>(null);
  const [healthOk, setHealthOk] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [st, hp] = await Promise.all([
          api.status(),
          api.health().then((r) => r.ok),
        ]);
        if (cancelled) return;
        setStatus(st);
        setHealthOk(hp);
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
    return <p className="text-red-300" role="alert">{error}</p>;
  }
  if (!status) {
    return <p className="text-zinc-500">Loading overview…</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Overview</h1>
        <p className="text-sm text-zinc-400">
          {network?.network_display_name ?? "Network"} · live coordinator data
        </p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card title="Coordinator">
          <StatusBadge label={healthOk ? "Reachable" : "Unreachable"} tone={healthOk ? "ok" : "bad"} />
          <p className="mt-2 text-xs text-zinc-500">/healthz on this host</p>
        </Card>
        <Card title="Devices">
          <p className="text-3xl font-semibold">{status.device_count}</p>
          <p className="text-sm text-zinc-400">
            {status.active_device_count} active · {status.revoked_device_count} revoked
          </p>
        </Card>
        <Card title="Signaling">
          <p className="text-3xl font-semibold">{status.signaling_online_count}</p>
          <p className="text-sm text-zinc-400">devices with active signaling session</p>
          <p className="mt-1 text-xs text-zinc-500">Not the same as WireGuard tunnel state.</p>
        </Card>
        <Card title="Enrollment">
          <StatusBadge
            label={status.legacy_enrollment ? "Legacy secret enabled" : "Invitation-based"}
            tone={status.legacy_enrollment ? "warn" : "ok"}
          />
        </Card>
        <Card title="Setup">
          <StatusBadge label={status.setup_complete ? "Complete" : "Incomplete"} tone={status.setup_complete ? "ok" : "warn"} />
        </Card>
        <Card title="Server">
          <p className="font-mono text-sm">{status.server_version}</p>
        </Card>
      </div>
    </div>
  );
}

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-4 light:border-zinc-200 light:bg-white">
      <h2 className="text-sm font-medium text-zinc-400">{title}</h2>
      <div className="mt-2">{children}</div>
    </section>
  );
}
