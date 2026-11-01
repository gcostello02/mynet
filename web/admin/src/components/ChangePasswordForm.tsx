import { useState } from "react";
import { ApiError, api } from "../api/client";
import { Button } from "./ui/Button";
import { Input } from "./ui/Input";

type Props = {
  onPasswordChanged: () => void;
};

export function ChangePasswordForm({ onPasswordChanged }: Props) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (next.length < 12) {
      setError("New password must be at least 12 characters.");
      return;
    }
    if (next !== confirm) {
      setError("New passwords do not match.");
      return;
    }
    setBusy(true);
    try {
      await api.changePassword(current, next);
      onPasswordChanged();
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Could not change password.");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="space-y-3 max-w-sm" onSubmit={(e) => void submit(e)}>
      <Input
        label="Current password"
        type="password"
        autoComplete="current-password"
        value={current}
        onChange={(e) => setCurrent(e.target.value)}
        required
      />
      <Input
        label="New password"
        type="password"
        autoComplete="new-password"
        value={next}
        onChange={(e) => setNext(e.target.value)}
        required
      />
      <Input
        label="Confirm new password"
        type="password"
        autoComplete="new-password"
        value={confirm}
        onChange={(e) => setConfirm(e.target.value)}
        required
      />
      {error && <p className="text-xs text-bad" role="alert">{error}</p>}
      <Button type="submit" size="sm" disabled={busy}>
        {busy ? "Updating…" : "Change password"}
      </Button>
      <p className="text-xs text-fg-subtle">
        All admin sessions are signed out after a successful change.
      </p>
    </form>
  );
}
