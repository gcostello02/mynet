import { useEffect, useState, type FormEvent } from "react";
import { ApiError, api, type Invitation } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";

export function InvitationsPage() {
  const [items, setItems] = useState<Invitation[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [label, setLabel] = useState("");
  const [hours, setHours] = useState(24);
  const [maxUses, setMaxUses] = useState(1);
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function load() {
    try {
      setItems(await api.invitations());
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to load invitations");
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setCreatedToken(null);
    try {
      const res = await api.createInvitation({
        label: label.trim() || "invite",
        expires_in_seconds: Math.max(3600, hours * 3600),
        max_uses: Math.max(1, maxUses),
      });
      setCreatedToken(res.enrollment_token);
      setLabel("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Create failed");
    } finally {
      setBusy(false);
    }
  }

  async function onRevoke(id: string) {
    if (!confirm("Revoke this invitation?")) return;
    try {
      await api.deleteInvitation(id);
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Revoke failed");
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Invitations</h1>
        <p className="text-sm text-zinc-400">Enrollment tokens are shown once at creation.</p>
      </div>
      {error && <p className="text-red-300" role="alert">{error}</p>}
      <form
        onSubmit={onCreate}
        className="grid gap-3 rounded-xl border border-zinc-800 bg-zinc-900/50 p-4 sm:grid-cols-4 light:border-zinc-200 light:bg-white"
      >
        <label className="text-sm sm:col-span-2">
          <span className="text-zinc-400">Label</span>
          <input
            className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 light:border-zinc-300 light:bg-white"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
          />
        </label>
        <label className="text-sm">
          <span className="text-zinc-400">Expires (hours)</span>
          <input
            type="number"
            min={1}
            className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 light:border-zinc-300 light:bg-white"
            value={hours}
            onChange={(e) => setHours(Number(e.target.value))}
          />
        </label>
        <label className="text-sm">
          <span className="text-zinc-400">Max uses</span>
          <input
            type="number"
            min={1}
            className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 light:border-zinc-300 light:bg-white"
            value={maxUses}
            onChange={(e) => setMaxUses(Number(e.target.value))}
          />
        </label>
        <button
          type="submit"
          disabled={busy}
          className="rounded-lg bg-sky-600 px-3 py-2 text-sm font-medium text-white hover:bg-sky-500 disabled:opacity-50 sm:col-span-4 sm:w-fit"
        >
          Create invitation
        </button>
      </form>
      {createdToken && (
        <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4">
          <p className="text-sm font-medium text-amber-100">Copy enrollment string now — it will not be shown again.</p>
          <textarea
            readOnly
            className="mt-2 w-full rounded-lg border border-zinc-700 bg-zinc-950 p-2 font-mono text-xs light:bg-white"
            rows={4}
            value={createdToken}
            onFocus={(e) => e.target.select()}
          />
          <button
            type="button"
            className="mt-2 text-sm text-sky-400 hover:underline"
            onClick={() => {
              void navigator.clipboard.writeText(createdToken);
            }}
          >
            Copy to clipboard
          </button>
        </div>
      )}
      {!items ? (
        <p className="text-zinc-500">Loading invitations…</p>
      ) : items.length === 0 ? (
        <p className="text-zinc-500">No invitations yet.</p>
      ) : (
        <ul className="space-y-2">
          {items.map((inv) => {
            const remaining = Math.max(0, inv.max_uses - inv.uses);
            const expired = new Date(inv.expires_at) < new Date();
            return (
              <li
                key={inv.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-zinc-800 px-4 py-3 light:border-zinc-200"
              >
                <div>
                  <p className="font-medium">{inv.label}</p>
                  <p className="text-xs text-zinc-500">
                    Uses {inv.uses}/{inv.max_uses} · {remaining} remaining · expires{" "}
                    {new Date(inv.expires_at).toLocaleString()}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  {inv.revoked && <StatusBadge label="Revoked" tone="bad" />}
                  {expired && !inv.revoked && <StatusBadge label="Expired" tone="muted" />}
                  {!inv.revoked && !expired && <StatusBadge label="Active" tone="ok" />}
                  {!inv.revoked && (
                    <button
                      type="button"
                      className="text-sm text-red-400 hover:underline"
                      onClick={() => void onRevoke(inv.id)}
                    >
                      Revoke
                    </button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
