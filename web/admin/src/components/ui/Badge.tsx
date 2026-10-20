type Tone = "success" | "warning" | "danger" | "neutral" | "info";

const tones: Record<Tone, string> = {
  success: "bg-emerald-500/12 text-emerald-400 ring-emerald-500/25",
  warning: "bg-amber-500/12 text-amber-400 ring-amber-500/25",
  danger: "bg-red-500/12 text-red-400 ring-red-500/25",
  neutral: "bg-zinc-500/12 text-zinc-400 ring-zinc-500/25",
  info: "bg-sky-500/12 text-sky-400 ring-sky-500/25",
};

export function Badge({ children, tone = "neutral" }: { children: React.ReactNode; tone?: Tone }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${tones[tone]}`}
    >
      {children}
    </span>
  );
}
