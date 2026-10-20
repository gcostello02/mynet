import { X } from "lucide-react";
import { useId, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useDialog } from "../../hooks/useDialog";
import { Button } from "./Button";

const widths = { sm: "sm:max-w-sm", md: "sm:max-w-md", lg: "sm:max-w-xl" };

export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = "md",
  dismissible = true,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  size?: keyof typeof widths;
  dismissible?: boolean;
}) {
  const titleId = useId();
  const descId = useId();
  const panelRef = useRef<HTMLDivElement>(null);
  useDialog(open, panelRef, () => dismissible && onClose());

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end justify-center sm:items-center sm:p-4">
      <div
        className="anim-fade absolute inset-0 bg-black/60"
        aria-hidden
        onClick={() => dismissible && onClose()}
      />
      <div
        ref={panelRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descId : undefined}
        className={`anim-pop relative flex max-h-[92vh] w-full flex-col rounded-t-xl border border-line-strong bg-panel shadow-pop outline-none sm:rounded-xl ${widths[size]}`}
      >
        <div className="flex items-start justify-between gap-4 px-5 pt-5">
          <div className="min-w-0">
            <h2 id={titleId} className="text-[15px] font-semibold tracking-tight">
              {title}
            </h2>
            {description && (
              <div id={descId} className="mt-1 text-[13px] leading-relaxed text-fg-muted">
                {description}
              </div>
            )}
          </div>
          {dismissible && (
            <Button variant="ghost" size="icon" aria-label="Close dialog" onClick={onClose} className="-mr-1.5 -mt-1">
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>
        <div className="overflow-y-auto px-5 py-4">{children}</div>
        {footer && (
          <div className="flex flex-col-reverse gap-2 border-t border-line bg-canvas/40 px-5 py-3 sm:flex-row sm:justify-end sm:rounded-b-xl">
            {footer}
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}
