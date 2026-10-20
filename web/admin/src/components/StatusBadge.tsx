type Tone = "ok" | "warn" | "bad" | "muted";

const tones: Record<Tone, string> = {
  ok: "bg-emerald-500/15 text-emerald-300 ring-emerald-500/30",
  warn: "bg-amber-500/15 text-amber-200 ring-amber-500/30",
  bad: "bg-red-500/15 text-red-200 ring-red-500/30",
  muted: "bg-zinc-500/15 text-zinc-300 ring-zinc-500/30",
};

export function StatusBadge({
  label,
  tone = "muted",
}: {
  label: string;
  tone?: Tone;
}) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${tones[tone]}`}
    >
      {label}
    </span>
  );
}
