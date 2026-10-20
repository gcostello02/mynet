import { useEffect, useState } from "react";
import { ApiError, api, type AdminStatus, type NetworkPublic } from "../api/client";

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

  if (error) return <p className="text-red-300" role="alert">{error}</p>;
  if (!status) return <p className="text-zinc-500">Loading settings…</p>;

  return (
    <div className="max-w-2xl space-y-6">
      <h1 className="text-2xl font-semibold">Settings</h1>
      <section className="rounded-xl border border-zinc-800 p-4 light:border-zinc-200">
        <h2 className="text-sm font-medium text-zinc-400">Coordinator identity</h2>
        <p className="mt-2 font-mono text-xs break-all">{status.coordinator_fingerprint}</p>
      </section>
      {network && (
        <section className="rounded-xl border border-zinc-800 p-4 light:border-zinc-200">
          <h2 className="text-sm font-medium text-zinc-400">Network</h2>
          <dl className="mt-2 space-y-2 text-sm">
            <div>
              <dt className="text-zinc-500">Display name</dt>
              <dd>{network.network_display_name}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Network ID</dt>
              <dd className="font-mono text-xs">{network.network_id}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Public URL</dt>
              <dd className="font-mono text-xs">{network.public_server_url}</dd>
            </div>
          </dl>
          <p className="mt-3 text-xs text-zinc-500">Network metadata is read-only in the dashboard.</p>
        </section>
      )}
      <section className="rounded-xl border border-zinc-800 p-4 light:border-zinc-200">
        <h2 className="text-sm font-medium text-zinc-400">Server</h2>
        <p className="mt-2 text-sm">Version: {status.server_version}</p>
        <p className="mt-1 text-sm">
          Legacy enrollment: {status.legacy_enrollment ? "enabled" : "disabled"} (change via CLI/API only)
        </p>
      </section>
    </div>
  );
}
