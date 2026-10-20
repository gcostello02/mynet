import { X } from "lucide-react";
import { useId, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useDialog } from "../../hooks/useDialog";
import { Button } from "./Button";

/** Right-hand inspector panel. */
export function Drawer({
  open,
  onClose,
  title,
  eyebrow,
  children,
  footer,
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  eyebrow?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
}) {
  const titleId = useId();
  const panelRef = useRef<HTMLElement>(null);
  useDialog(open, panelRef, onClose);

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-40 flex justify-end">
      <div className="anim-fade absolute inset-0 bg-black/50" aria-hidden onClick={onClose} />
      <aside
        ref={panelRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="anim-slide relative flex h-full w-full flex-col border-l border-line-strong bg-panel shadow-pop outline-none sm:max-w-[420px]"
      >
        <div className="flex items-start justify-between gap-3 border-b border-line px-5 py-4">
          <div className="min-w-0">
            {eyebrow && <div className="mb-1 text-[11px] font-medium uppercase tracking-wider text-fg-subtle">{eyebrow}</div>}
            <h2 id={titleId} className="truncate text-[15px] font-semibold tracking-tight">
              {title}
            </h2>
          </div>
          <Button variant="ghost" size="icon" aria-label="Close panel" onClick={onClose} className="-mr-1.5 -mt-1">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto px-5 py-5">{children}</div>
        {footer && <div className="border-t border-line px-5 py-3">{footer}</div>}
      </aside>
    </div>,
    document.body,
  );
}
