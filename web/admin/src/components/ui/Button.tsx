import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger";

const variants: Record<Variant, string> = {
  primary:
    "bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] border-transparent",
  secondary:
    "bg-[var(--color-surface-overlay)] text-[var(--color-fg)] border-[var(--color-border)] hover:bg-[var(--color-surface-raised)]",
  ghost:
    "bg-transparent text-[var(--color-fg-muted)] border-transparent hover:bg-[var(--color-surface-overlay)] hover:text-[var(--color-fg)]",
  danger: "bg-red-600 text-white hover:bg-red-500 border-transparent",
};

type Size = "sm" | "md";

export function Button({
  variant = "secondary",
  size = "md",
  className = "",
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
  children: ReactNode;
}) {
  const sz = size === "sm" ? "h-8 px-3 text-xs" : "h-9 px-4 text-sm";
  return (
    <button
      type="button"
      className={`inline-flex items-center justify-center gap-2 rounded-[var(--radius-md)] border font-medium transition-colors disabled:opacity-50 disabled:pointer-events-none ${sz} ${variants[variant]} ${className}`}
      {...props}
    />
  );
}
