import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface-overlay)]">
        <Icon className="h-5 w-5 text-[var(--color-fg-subtle)]" aria-hidden />
      </div>
      <h3 className="text-sm font-medium text-[var(--color-fg)]">{title}</h3>
      <p className="mt-1 max-w-sm text-sm text-[var(--color-fg-muted)]">{description}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
