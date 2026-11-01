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
    <div className="flex flex-col items-center px-6 py-14 text-center">
      <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-line-strong bg-raised text-fg-muted">
        <Icon className="h-[18px] w-[18px]" aria-hidden />
      </div>
      <h3 className="mt-4 text-[13px] font-medium text-fg">{title}</h3>
      <p className="mt-1 max-w-sm text-[13px] leading-relaxed text-fg-muted">{description}</p>
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}
