import { Loader2 } from "lucide-react";
import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "danger-soft";
type Size = "sm" | "md" | "icon";

const variants: Record<Variant, string> = {
  primary:
    "bg-accent-solid text-accent-solid-fg hover:bg-accent-solid-hover active:brightness-95 border-transparent",
  secondary:
    "bg-raised text-fg border-line-strong hover:bg-hover hover:border-line-strong active:bg-line",
  ghost: "bg-transparent text-fg-muted border-transparent hover:bg-hover hover:text-fg active:bg-line",
  danger:
    "bg-bad-solid text-white hover:bg-bad-solid-hover active:brightness-95 border-transparent",
  "danger-soft":
    "bg-bad-soft text-bad border-transparent hover:bg-bad-soft hover:brightness-110 active:brightness-95",
};

const sizes: Record<Size, string> = {
  sm: "h-7 gap-1.5 px-2.5 text-xs",
  md: "h-8 gap-2 px-3 text-[13px]",
  icon: "h-7 w-7 justify-center text-xs",
};

export function Button({
  variant = "secondary",
  size = "md",
  loading = false,
  className = "",
  children,
  disabled,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
  children?: ReactNode;
}) {
  return (
    <button
      type="button"
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={`inline-flex shrink-0 select-none items-center justify-center whitespace-nowrap rounded-md border font-medium transition-[background-color,border-color,color,filter] duration-100 disabled:pointer-events-none disabled:opacity-50 ${sizes[size]} ${variants[variant]} ${className}`}
      {...props}
    >
      {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />}
      {children}
    </button>
  );
}
