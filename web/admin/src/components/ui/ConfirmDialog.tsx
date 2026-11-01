import { useState, type ReactNode } from "react";
import { Button } from "./Button";
import { Input } from "./Input";
import { Modal } from "./Modal";

/**
 * Accessible replacement for window.confirm().
 * `requireText` forces the operator to type a value before a destructive action.
 */
export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  busy,
  requireText,
  onConfirm,
  onCancel,
  children,
}: {
  open: boolean;
  title: string;
  description: ReactNode;
  confirmLabel: string;
  busy?: boolean;
  requireText?: string;
  onConfirm: () => void;
  onCancel: () => void;
  children?: ReactNode;
}) {
  return (
    <Modal open={open} onClose={() => !busy && onCancel()} title={title} description={description} size="sm">
      <ConfirmBody
        key={String(open)}
        confirmLabel={confirmLabel}
        busy={busy}
        requireText={requireText}
        onConfirm={onConfirm}
        onCancel={onCancel}
      >
        {children}
      </ConfirmBody>
    </Modal>
  );
}

function ConfirmBody({
  confirmLabel,
  busy,
  requireText,
  onConfirm,
  onCancel,
  children,
}: {
  confirmLabel: string;
  busy?: boolean;
  requireText?: string;
  onConfirm: () => void;
  onCancel: () => void;
  children?: ReactNode;
}) {
  const [typed, setTyped] = useState("");
  const ok = !requireText || typed === requireText;
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (ok && !busy) onConfirm();
      }}
    >
      {children}
      {requireText && (
        <div className="mt-3">
          <Input
            label={`Type “${requireText}” to confirm`}
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            data-autofocus
          />
        </div>
      )}
      <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button onClick={onCancel} disabled={busy} {...(requireText ? {} : { "data-autofocus": true })}>
          Cancel
        </Button>
        <Button variant="danger" loading={busy} disabled={!ok} {...{ type: "submit" }}>
          {confirmLabel}
        </Button>
      </div>
    </form>
  );
}
