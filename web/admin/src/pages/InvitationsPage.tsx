import { AlertTriangle, Plus, Ticket, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { useSearchParams } from "react-router-dom";
import { ApiError, api, type Invitation } from "../api/client";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Panel } from "../components/ui/Card";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { CopyButton } from "../components/ui/Copy";
import { EmptyState } from "../components/ui/EmptyState";
import { Input, Select } from "../components/ui/Input";
import { Modal } from "../components/ui/Modal";
import { PageHeader } from "../components/ui/PageHeader";
import { RowsSkeleton } from "../components/ui/Skeleton";
import { useToast } from "../hooks/useToast";
import { formatDateTime, relativeTime } from "../lib/time";

const EXPIRY: { label: string; seconds: number }[] = [
  { label: "1 hour", seconds: 3600 },
  { label: "24 hours", seconds: 86400 },
  { label: "7 days", seconds: 7 * 86400 },
  { label: "30 days", seconds: 30 * 86400 },
];

type State = "active" | "used" | "expired" | "revoked";

function stateOf(i: Invitation, now = Date.now()): State {
  if (i.revoked) return "revoked";
  if (i.uses >= i.max_uses) return "used";
  if (new Date(i.expires_at).getTime() <= now) return "expired";
  return "active";
}

function StateBadge({ s }: { s: State }) {
  if (s === "active") return <Badge tone="ok">Active</Badge>;
  if (s === "used") return <Badge tone="neutral">Fully used</Badge>;
  if (s === "expired") return <Badge tone="warn">Expired</Badge>;
  return <Badge tone="bad">Revoked</Badge>;
}

function Usage({ i }: { i: Invitation }) {
  const pct = Math.min(100, Math.round((i.uses / Math.max(i.max_uses, 1)) * 100));
  return (
    <div className="flex items-center gap-2.5">
      <div className="h-1 w-14 overflow-hidden rounded-full bg-line-strong" aria-hidden>
        <div className="h-full rounded-full bg-accent" style={{ width: `${pct}%` }} />
      </div>
      <span className="font-mono text-xs tabular-nums text-fg-muted">
        {i.uses} / {i.max_uses}
      </span>
    </div>
  );
}

type Created = { token: string; label: string };

export function InvitationsPage() {
  const { push } = useToast();
  const [params, setParams] = useSearchParams();
  const [items, setItems] = useState<Invitation[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(params.get("new") === "1");
  const [label, setLabel] = useState("");
  const [expiry, setExpiry] = useState(86400);
  const [maxUses, setMaxUses] = useState(1);
  const [creating, setCreating] = useState(false);
  const [createErr, setCreateErr] = useState<string | null>(null);
  const [created, setCreated] = useState<Created | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<Invitation | null>(null);
  const [revoking, setRevoking] = useState(false);

  const load = useCallback(async () => {
    try {
      setItems(await api.invitations());
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not load invitations.");
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);

  // The ?new=1 deep link is consumed once so a refresh doesn't reopen the dialog.
  useEffect(() => {
    if (params.get("new") === "1") {
      const next = new URLSearchParams(params);
      next.delete("new");
      setParams(next, { replace: true });
    }
  }, [params, setParams]);

  const sorted = useMemo(
    () => [...(items ?? [])].sort((a, b) => b.created_at.localeCompare(a.created_at)),
    [items],
  );

  const expiryLabel = EXPIRY.find((e) => e.seconds === expiry)?.label ?? "";
  const summary = `Valid for ${expiryLabel}, usable ${maxUses === 1 ? "once" : `up to ${maxUses} times`}.`;

  function openCreate() {
    setLabel("");
    setExpiry(86400);
    setMaxUses(1);
    setCreateErr(null);
    setCreateOpen(true);
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    setCreating(true);
    setCreateErr(null);
    try {
      const res = await api.createInvitation({
        label: label.trim(),
        expires_in_seconds: expiry,
        max_uses: maxUses,
      });
      // Held in component state only — never written to storage, URLs, or logs.
      setCreated({ token: res.enrollment_token, label: label.trim() });
      setCreateOpen(false);
      void load();
    } catch (err) {
      setCreateErr(err instanceof ApiError ? err.message : "Could not create the invitation.");
    } finally {
      setCreating(false);
    }
  }

  async function onRevoke() {
    if (!revokeTarget) return;
    setRevoking(true);
    try {
      await api.deleteInvitation(revokeTarget.id);
      push("Invitation revoked.", "success");
      setRevokeTarget(null);
      await load();
    } catch (e) {
      push(e instanceof ApiError ? e.message : "Could not revoke the invitation.", "error");
    } finally {
      setRevoking(false);
    }
  }

  return (
    <div>
      <PageHeader
        title="Invitations"
        description="An invitation is a one-time secret that lets a new device enroll. Create one, hand it to the person or device you trust, and revoke it any time before it’s used."
        action={
          <Button variant="primary" onClick={openCreate}>
            <Plus className="h-3.5 w-3.5" aria-hidden /> Create invitation
          </Button>
        }
      />

      {error && (
        <p role="alert" className="mb-3 rounded-md border border-bad/30 bg-bad-soft px-3 py-2 text-[13px] text-bad">
          {error}
        </p>
      )}

      <Panel className="overflow-hidden">
        {items === null && !error ? (
          <RowsSkeleton rows={4} />
        ) : sorted.length === 0 ? (
          <EmptyState
            icon={Ticket}
            title="No invitations yet"
            description="Create an invitation to enroll your first device. Invitation secrets are shown only once."
            action={
              <Button variant="primary" onClick={openCreate}>
                <Plus className="h-3.5 w-3.5" /> Create invitation
              </Button>
            }
          />
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full text-left text-[13px]">
                <thead>
                  <tr className="border-b border-line text-xs text-fg-muted">
                    <th scope="col" className="px-4 py-2.5 font-medium">Label</th>
                    <th scope="col" className="px-4 py-2.5 font-medium">Status</th>
                    <th scope="col" className="px-4 py-2.5 font-medium">Uses</th>
                    <th scope="col" className="px-4 py-2.5 font-medium">Expires</th>
                    <th scope="col" className="px-4 py-2.5 font-medium">Created</th>
                    <th scope="col" className="w-12 px-2 py-2.5"><span className="sr-only">Actions</span></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {sorted.map((i) => {
                    const s = stateOf(i);
                    return (
                      <tr key={i.id} className="transition-colors hover:bg-hover/60">
                        <td className="px-4 py-2.5">
                          <span className={i.label ? "font-medium" : "text-fg-subtle"}>{i.label || "Untitled"}</span>
                          <span className="ml-2 font-mono text-[11px] text-fg-subtle">{i.id.slice(0, 8)}</span>
                        </td>
                        <td className="px-4 py-2.5"><StateBadge s={s} /></td>
                        <td className="px-4 py-2.5"><Usage i={i} /></td>
                        <td className="px-4 py-2.5 text-fg-muted" title={formatDateTime(i.expires_at)}>
                          {s === "revoked" || s === "used" ? "—" : relativeTime(i.expires_at)}
                        </td>
                        <td className="px-4 py-2.5 text-fg-muted" title={formatDateTime(i.created_at)}>
                          {relativeTime(i.created_at)}
                        </td>
                        <td className="px-2 py-2.5 text-right">
                          {s === "active" && (
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={`Revoke invitation ${i.label || i.id.slice(0, 8)}`}
                              onClick={() => setRevokeTarget(i)}
                            >
                              <Trash2 className="h-3.5 w-3.5" aria-hidden />
                            </Button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            <ul className="divide-y divide-line md:hidden">
              {sorted.map((i) => {
                const s = stateOf(i);
                return (
                  <li key={i.id} className="flex items-center gap-3 px-4 py-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate text-[13px] font-medium">{i.label || "Untitled"}</span>
                        <StateBadge s={s} />
                      </div>
                      <div className="mt-1.5 flex items-center gap-3 text-xs text-fg-muted">
                        <Usage i={i} />
                        {s === "active" && <span>expires {relativeTime(i.expires_at)}</span>}
                      </div>
                    </div>
                    {s === "active" && (
                      <Button variant="ghost" size="icon" aria-label="Revoke invitation" onClick={() => setRevokeTarget(i)}>
                        <Trash2 className="h-4 w-4" aria-hidden />
                      </Button>
                    )}
                  </li>
                );
              })}
            </ul>
          </>
        )}
      </Panel>

      <Modal
        open={createOpen}
        onClose={() => !creating && setCreateOpen(false)}
        title="Create invitation"
        description="Generates a one-time enrollment secret. You’ll see it once, right after creating."
      >
        <form onSubmit={onCreate} className="space-y-4">
          <Input
            label="Label (optional)"
            hint="Only for your reference — e.g. who it’s for."
            data-autofocus
            maxLength={80}
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="Sam’s laptop"
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <Select
              label="Expires after"
              hint="Unused invitations stop working after this."
              value={expiry}
              onChange={(e) => setExpiry(Number(e.target.value))}
            >
              {EXPIRY.map((e) => (
                <option key={e.seconds} value={e.seconds}>{e.label}</option>
              ))}
            </Select>
            <Input
              label="Maximum uses"
              hint="Devices that may enroll with it."
              type="number"
              inputMode="numeric"
              min={1}
              max={100}
              value={maxUses}
              onChange={(e) => setMaxUses(Math.max(1, Math.min(100, Number(e.target.value) || 1)))}
              className="font-mono tabular-nums"
            />
          </div>
          <p className="rounded-md border border-line bg-canvas px-3 py-2 text-xs text-fg-muted">{summary}</p>
          {createErr && (
            <p role="alert" className="rounded-md border border-bad/30 bg-bad-soft px-3 py-2 text-[13px] text-bad">
              {createErr}
            </p>
          )}
          <div className="flex justify-end gap-2 pt-1">
            <Button onClick={() => setCreateOpen(false)} disabled={creating}>Cancel</Button>
            <Button variant="primary" type="submit" loading={creating}>Create invitation</Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={!!created}
        onClose={() => setCreated(null)}
        title="Invitation created"
        description={created?.label ? `For ${created.label}.` : undefined}
        size="lg"
        dismissible={false}
        footer={
          <div className="flex w-full flex-wrap items-center justify-between gap-2">
            <Button variant="ghost" onClick={() => setCreated(null)}>Close without copying</Button>
            {created && <CopyButton variant="primary" size="md" value={created.token} label="Copy secret" />}
          </div>
        }
      >
        {created && (
          <div className="space-y-4">
            <div>
              <div className="mb-1.5 text-xs font-medium text-fg-muted">Enrollment secret</div>
              <code
                data-autofocus
                tabIndex={0}
                className="block max-h-32 overflow-y-auto break-all rounded-md border border-line-strong bg-canvas p-3 font-mono text-xs leading-5 text-fg"
              >
                {created.token}
              </code>
            </div>
            <div className="flex gap-2.5 rounded-md border border-warn/30 bg-warn-soft px-3 py-2.5 text-[13px] leading-relaxed text-warn">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
              <div>
                <p className="font-medium">This is the only time you’ll see this secret.</p>
                <p className="mt-1 text-warn/90">
                  The coordinator stores only a hash, so it can’t be shown again. Anyone who has it can enroll a
                  device, subject to this invitation’s expiry and use limit. Share it only with someone you trust.
                  If it’s lost, revoke it and create a new one.
                </p>
              </div>
            </div>
            <p className="text-xs text-fg-muted">
              On the new device, run <code className="rounded bg-raised px-1 py-0.5 font-mono">mynet join</code> and
              paste the secret.
            </p>
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={!!revokeTarget}
        title="Revoke invitation?"
        description="It will stop working immediately. Devices already enrolled with it keep their access."
        confirmLabel="Revoke invitation"
        busy={revoking}
        onConfirm={() => void onRevoke()}
        onCancel={() => setRevokeTarget(null)}
      />
    </div>
  );
}
