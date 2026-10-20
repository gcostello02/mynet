import { useEffect, useState } from "react";
import { ApiError, api, type AdminStatus, type NetworkPublic } from "../api/client";
import { PageHeader } from "../components/ui/PageHeader";
import { Card, CardHeader } from "../components/ui/Card";
import { Skeleton } from "../components/ui/Skeleton";
import { CopyButton } from "../components/ui/CopyButton";
import { Badge } from "../components/ui/Badge";

export function SettingsPage() {
  const [status, setStatus] = useState<AdminStatus | null>(null);
  const [network, setNetwork] = useState<NetworkPublic | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const st = await api.status();
        setStatus(st);
        try {
          setNetwork(await api.networkPublic());
        } catch {
          /* not initialized */
        }
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Failed to load settings");
      }
    })();
  }, []);

  if (error) {
    return (
      <p className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300" role="alert">
        {error}
      </p>
    );
  }

  return (
    <div>
      <PageHeader
        title="Settings"
        description="Network identity and coordinator metadata (read-only in the dashboard)."
      />

      <div className="grid max-w-3xl gap-6">
        <Card>
          <CardHeader title="Coordinator identity" description="Fingerprint pins enrollment to this coordinator." />
          {!status ? (
            <Skeleton className="h-16" />
          ) : (
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <p className="break-all font-mono text-xs text-[var(--color-fg-muted)]">
                {status.coordinator_fingerprint}
              </p>
              <CopyButton value={status.coordinator_fingerprint} label="Copy" />
            </div>
          )}
        </Card>

        {network && (
          <Card>
            <CardHeader title="Network" />
            <dl className="grid gap-4 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-xs text-[var(--color-fg-subtle)]">Display name</dt>
                <dd className="mt-1 font-medium">{network.network_display_name}</dd>
              </div>
              <div>
                <dt className="text-xs text-[var(--color-fg-subtle)]">Public URL</dt>
                <dd className="mt-1 flex flex-wrap items-center gap-2">
                  <span className="font-mono text-xs">{network.public_server_url}</span>
                  <CopyButton value={network.public_server_url} label="Copy URL" />
                </dd>
              </div>
              <div className="sm:col-span-2">
                <dt className="text-xs text-[var(--color-fg-subtle)]">Network ID</dt>
                <dd className="mt-1 break-all font-mono text-xs">{network.network_id}</dd>
              </div>
            </dl>
          </Card>
        )}

        <Card>
          <CardHeader title="Server & enrollment" />
          {!status ? (
            <Skeleton className="h-20" />
          ) : (
            <dl className="space-y-3 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <dt className="text-[var(--color-fg-subtle)]">Version</dt>
                <dd className="font-mono text-xs">{status.server_version}</dd>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <dt className="text-[var(--color-fg-subtle)]">Legacy enrollment</dt>
                <dd>
                  <Badge tone={status.legacy_enrollment ? "warning" : "success"}>
                    {status.legacy_enrollment ? "Enabled" : "Disabled"}
                  </Badge>
                </dd>
              </div>
              <p className="text-xs text-[var(--color-fg-subtle)]">
                Changing legacy enrollment or network metadata requires the CLI or API. Coordinator private keys are never exposed here.
              </p>
            </dl>
          )}
        </Card>

        <Card>
          <CardHeader title="Browser session" description="Security properties of this admin UI." />
          <ul className="list-inside list-disc space-y-1 text-sm text-[var(--color-fg-muted)]">
            <li>HttpOnly session cookie with CSRF on mutations</li>
            <li>No admin or invitation secrets in localStorage</li>
            <li>Same-origin API; content served only from this coordinator</li>
          </ul>
        </Card>
      </div>
    </div>
  );
}
