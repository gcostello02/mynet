import type { InputHTMLAttributes } from "react";

export function Input({
  label,
  hint,
  className = "",
  id,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
  hint?: string;
}) {
  const inputId = id ?? (label ? label.replace(/\s+/g, "-").toLowerCase() : undefined);
  return (
    <label className="block text-sm">
      {label && (
        <span className="mb-1.5 block font-medium text-[var(--color-fg-muted)]">{label}</span>
      )}
      <input
        id={inputId}
        className={`w-full rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] ${className}`}
        {...props}
      />
      {hint && <span className="mt-1 block text-xs text-[var(--color-fg-subtle)]">{hint}</span>}
    </label>
  );
}

export function Select({
  label,
  children,
  className = "",
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement> & { label?: string }) {
  return (
    <label className="block text-sm">
      {label && (
        <span className="mb-1.5 block font-medium text-[var(--color-fg-muted)]">{label}</span>
      )}
      <select
        className={`w-full rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-fg)] ${className}`}
        {...props}
      >
        {children}
      </select>
    </label>
  );
}
