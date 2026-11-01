import { Check, Copy } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useToast } from "../../hooks/useToast";
import { Button } from "./Button";

export function useCopy() {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  const { push } = useToast();

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setCopied(false), 1800);
      return true;
    } catch {
      push("Could not access the clipboard. Select the text and copy manually.", "error");
      return false;
    }
  }
  return { copied, copy };
}

export function CopyButton({
  value,
  label = "Copy",
  size = "sm",
  variant = "secondary",
}: {
  value: string;
  label?: string;
  size?: "sm" | "md";
  variant?: "secondary" | "primary" | "ghost";
}) {
  const { copied, copy } = useCopy();
  return (
    <Button variant={variant} size={size} onClick={() => void copy(value)}>
      {copied ? <Check className="h-3.5 w-3.5 text-ok" aria-hidden /> : <Copy className="h-3.5 w-3.5" aria-hidden />}
      <span aria-live="polite">{copied ? "Copied" : label}</span>
    </Button>
  );
}

/** Read-only identifier with inline copy. */
export function CopyField({
  label,
  value,
  mono = true,
  truncate = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
  truncate?: boolean;
}) {
  const { copied, copy } = useCopy();
  return (
    <div>
      <div className="mb-1.5 text-xs font-medium text-fg-muted">{label}</div>
      <div className="flex items-stretch overflow-hidden rounded-md border border-line-strong bg-canvas">
        <code
          className={`min-w-0 flex-1 px-2.5 py-1.5 text-xs leading-5 text-fg ${mono ? "font-mono" : ""} ${
            truncate ? "truncate" : "break-all"
          }`}
          title={value}
        >
          {value || "—"}
        </code>
        <button
          type="button"
          aria-label={`Copy ${label}`}
          onClick={() => void copy(value)}
          disabled={!value}
          className="flex w-8 shrink-0 items-center justify-center border-l border-line-strong text-fg-muted transition-colors hover:bg-hover hover:text-fg disabled:opacity-40"
        >
          {copied ? <Check className="h-3.5 w-3.5 text-ok" /> : <Copy className="h-3.5 w-3.5" />}
        </button>
      </div>
    </div>
  );
}
