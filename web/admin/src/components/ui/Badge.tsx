import type { ReactNode } from "react";

export type Tone = "ok" | "warn" | "bad" | "neutral" | "accent";

const dot: Record<Tone, string> = {
  ok: "bg-ok",
  warn: "bg-warn",
  bad: "bg-bad",
  neutral: "bg-fg-subtle",
  accent: "bg-accent",
};

const chip: Record<Tone, string> = {
  ok: "bg-ok-soft text-ok",
  warn: "bg-warn-soft text-warn",
  bad: "bg-bad-soft text-bad",
  neutral: "bg-hover text-fg-muted",
  accent: "bg-accent-soft text-accent",
};

export function StatusDot({ tone, hollow = false }: { tone: Tone; hollow?: boolean }) {
  return (
    <span
      aria-hidden
      className={`inline-block h-1.5 w-1.5 shrink-0 rounded-full ${
        hollow ? `border border-fg-subtle bg-transparent` : dot[tone]
      }`}
    />
  );
}

/** Pill with a leading status dot. */
export function Badge({
  children,
  tone = "neutral",
  dotted = true,
}: {
  children: ReactNode;
  tone?: Tone;
  dotted?: boolean;
}) {
  return (
    <span
      className={`inline-flex h-5 items-center gap-1.5 whitespace-nowrap rounded-full px-2 text-[11px] font-medium ${chip[tone]}`}
    >
      {dotted && <StatusDot tone={tone} />}
      {children}
    </span>
  );
}

/** Plain text status with dot, for dense tables. */
export function StatusText({
  tone,
  hollow,
  children,
}: {
  tone: Tone;
  hollow?: boolean;
  children: ReactNode;
}) {
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap text-[13px] text-fg">
      <StatusDot tone={tone} hollow={hollow} />
      {children}
    </span>
  );
}
