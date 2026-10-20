import { useCallback, useEffect, useState } from "react";
import { Plus, Ticket } from "lucide-react";
import { ApiError, api, type Invitation } from "../api/client";
import { PageHeader } from "../components/ui/PageHeader";
import { Card } from "../components/ui/Card";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Input } from "../components/ui/Input";
import { Modal } from "../components/ui/Modal";
import { EmptyState } from "../components/ui/EmptyState";
import { TableSkeleton } from "../components/ui/Skeleton";
import { CopyButton } from "../components/ui/CopyButton";
import { useToast } from "../hooks/useToast";

export function InvitationsPage() {
  const { push } = useToast();
  const [items, setItems] = useState<Invitation[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [label, setLabel] = useState("");
  const [hours, setHours] = useState(24);
  const [maxUses, setMaxUses] = useState(1);
  const [busy, setBusy] = useState(false);
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [tokenModalOpen, setTokenModalOpen] = useState(false);

  const load = useCallback(async () => {
    try {
      setItems(await api.invitations());
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to load invitations");
    }
  }, []);

  useEffect(() => {
    void load();
    return () => setCreatedToken(null);
  }, [load]);

  async function onCreate() {
    setBusy(true);
    setCreatedToken(null);
    try {
      const res = await api.createInvitation({
        label: label.trim() || "invite",
        expires_in_seconds: Math.max(3600, hours * 3600),
        max_uses: Math.max(1, maxUses),
      });
      setCreatedToken(res.enrollment_token);
      setTokenModalOpen(true);
      setCreateOpen(false);
      setLabel("");
      push("Invitation created.");
      await load();
    } catch (err) {
      push(err instanceof ApiError ? err.message : "Create failed", "error");
    } finally {
      setBusy(false);
    }
  }

  async function onRevoke(id: string) {
    if (!confirm("Revoke this invitation? Unused tokens will stop working.")) return;
    try {
      await api.deleteInvitation(id);
      push("Invitation revoked.");
      await load();
    } catch (e) {
      push(e instanceof ApiError ? e.message : "Revoke failed", "error");
    }
  }

  function invitationStatus(inv: Invitation) {
    if (inv.revoked) return { label: "Revoked", tone: "danger" as const };
    if (new Date(inv.expires_at) < new Date()) return { label: "Expired", tone: "neutral" as const };
    if (inv.uses >= inv.max_uses) return { label: "Exhausted", tone: "warning" as const };
    return { label: "Active", tone: "success" as const };
  }

  return (
    <div>
      <PageHeader
        title="Invitations"
        description="One-time enrollment tokens. Secrets are shown only immediately after creation."
        action={
          <Button variant="primary" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" />
            Create invitation
          </Button>
        }
      />

      {error && (
        <p className="mb-4 rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300" role="alert">
          {error}
        </p>
      )}

      <Card padding={false}>
        {!items ? (
          <div className="p-4">
            <TableSkeleton rows={4} />
          </div>
        ) : items.length === 0 ? (
          <EmptyState
            icon={Ticket}
            title="No invitations"
            description="Create an invitation to enroll a new device securely."
            action={
              <Button variant="primary" onClick={() => setCreateOpen(true)}>
                Create invitation
              </Button>
            }
          />
        ) : (
          <ul className="divide-y divide-[var(--color-border)]">
            {items.map((inv) => {
              const st = invitationStatus(inv);
              const remaining = Math.max(0, inv.max_uses - inv.uses);
              return (
                <li key={inv.id} className="flex flex-col gap-2 px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <p className="font-medium">{inv.label}</p>
                    <p className="mt-0.5 text-xs text-[var(--color-fg-muted)]">
                      {inv.uses} / {inv.max_uses} uses · {remaining} remaining · expires{" "}
                      {new Date(inv.expires_at).toLocaleString()}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge tone={st.tone}>{st.label}</Badge>
                    {!inv.revoked && st.label === "Active" && (
                      <Button variant="ghost" size="sm" onClick={() => void onRevoke(inv.id)}>
                        Revoke
                      </Button>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </Card>

      <Modal
        open={createOpen}
        onClose={() => !busy && setCreateOpen(false)}
        title="Create invitation"
        description="Configure label, lifetime, and how many devices may enroll."
        footer={
          <>
            <Button variant="secondary" onClick={() => setCreateOpen(false)} disabled={busy}>
              Cancel
            </Button>
            <Button variant="primary" onClick={() => void onCreate()} disabled={busy}>
              {busy ? "Creating…" : "Create"}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input label="Label" placeholder="e.g. laptop" value={label} onChange={(e) => setLabel(e.target.value)} />
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Expires (hours)"
              type="number"
              min={1}
              value={hours}
              onChange={(e) => setHours(Number(e.target.value))}
            />
            <Input
              label="Max uses"
              type="number"
              min={1}
              value={maxUses}
              onChange={(e) => setMaxUses(Number(e.target.value))}
            />
          </div>
        </div>
      </Modal>

      <Modal
        open={tokenModalOpen && createdToken !== null}
        onClose={() => {
          setTokenModalOpen(false);
          setCreatedToken(null);
        }}
        title="Enrollment string"
        description="Copy this now. It cannot be retrieved again from the coordinator."
        size="lg"
        footer={
          <Button
            variant="primary"
            onClick={() => {
              setTokenModalOpen(false);
              setCreatedToken(null);
            }}
          >
            Done
          </Button>
        }
      >
        <div className="space-y-3">
          <textarea
            readOnly
            className="w-full rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-3 font-mono text-xs leading-relaxed"
            rows={5}
            value={createdToken ?? ""}
            onFocus={(e) => e.target.select()}
            aria-label="Enrollment token"
          />
          {createdToken && <CopyButton value={createdToken} label="Copy enrollment string" />}
          <p className="text-xs text-[var(--color-fg-subtle)]">
            Treat this like a password. Do not share in chat or email. Run{" "}
            <code className="rounded bg-[var(--color-surface-overlay)] px-1">mynet join</code> on the device.
          </p>
        </div>
      </Modal>
    </div>
  );
}
