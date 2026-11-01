export function Segmented<T extends string | number>({
  label,
  value,
  onChange,
  options,
  size = "md",
}: {
  label: string;
  value: T;
  onChange: (v: T) => void;
  options: { value: T; label: string; count?: number }[];
  size?: "sm" | "md";
}) {
  return (
    <div
      role="group"
      aria-label={label}
      className="inline-flex rounded-md border border-line-strong bg-canvas p-0.5"
    >
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={String(o.value)}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(o.value)}
            className={`inline-flex items-center gap-1.5 rounded px-2.5 font-medium transition-colors ${
              size === "sm" ? "h-6 text-xs" : "h-7 text-[13px]"
            } ${
              active
                ? "bg-raised text-fg shadow-[0_0_0_1px_var(--color-line-strong)]"
                : "text-fg-muted hover:text-fg"
            }`}
          >
            {o.label}
            {o.count !== undefined && (
              <span className="font-mono text-[11px] tabular-nums text-fg-subtle">{o.count}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}
