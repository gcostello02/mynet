import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";

export type MenuItem =
  | { type?: "item"; label: string; icon?: ReactNode; danger?: boolean; disabled?: boolean; onSelect: () => void }
  | { type: "separator" }
  | { type: "label"; label: string };

/** Dropdown menu rendered in a portal so table overflow never clips it. */
export function Menu({
  label,
  trigger,
  triggerClassName,
  items,
  align = "right",
}: {
  label: string;
  trigger: ReactNode;
  triggerClassName?: string;
  items: MenuItem[];
  align?: "left" | "right";
}) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState({ top: 0, left: 0, right: 0 });
  const btnRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const id = useId();

  const close = useCallback((restore = true) => {
    setOpen(false);
    if (restore) btnRef.current?.focus();
  }, []);

  const toggle = () => {
    if (!open && btnRef.current) {
      const r = btnRef.current.getBoundingClientRect();
      setPos({ top: r.bottom + 6, left: r.left, right: window.innerWidth - r.right });
    }
    setOpen((o) => !o);
  };

  useEffect(() => {
    if (!open) return;
    const buttons = () => Array.from(listRef.current?.querySelectorAll<HTMLButtonElement>("button:not([disabled])") ?? []);
    buttons()[0]?.focus();
    const onDown = (e: MouseEvent) => {
      if (!listRef.current?.contains(e.target as Node) && !btnRef.current?.contains(e.target as Node)) close(false);
    };
    const onKey = (e: KeyboardEvent) => {
      const list = buttons();
      const idx = list.indexOf(document.activeElement as HTMLButtonElement);
      if (e.key === "Escape") {
        e.preventDefault();
        close();
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        list[(idx + 1) % list.length]?.focus();
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        list[(idx - 1 + list.length) % list.length]?.focus();
      } else if (e.key === "Tab") {
        close(false);
      }
    };
    const onScroll = () => close(false);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    window.addEventListener("resize", onScroll);
    window.addEventListener("scroll", onScroll, true);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", onScroll);
      window.removeEventListener("scroll", onScroll, true);
    };
  }, [open, close]);

  return (
    <>
      <button
        ref={btnRef}
        type="button"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        onClick={(e) => {
          e.stopPropagation();
          toggle();
        }}
        className={triggerClassName}
      >
        {trigger}
      </button>
      {open &&
        createPortal(
          <div
            ref={listRef}
            id={id}
            role="menu"
            aria-label={label}
            style={align === "right" ? { top: pos.top, right: pos.right } : { top: pos.top, left: pos.left }}
            className="anim-pop fixed z-[60] min-w-48 rounded-lg border border-line-strong bg-raised p-1 shadow-pop"
          >
            {items.map((it, i) => {
              if (it.type === "separator") return <div key={i} role="separator" className="my-1 h-px bg-line" />;
              if (it.type === "label")
                return (
                  <div key={i} className="px-2.5 pb-1 pt-1.5 text-[11px] font-medium uppercase tracking-wider text-fg-subtle">
                    {it.label}
                  </div>
                );
              return (
                <button
                  key={i}
                  role="menuitem"
                  type="button"
                  disabled={it.disabled}
                  onClick={() => {
                    close();
                    it.onSelect();
                  }}
                  className={`flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left text-[13px] outline-none transition-colors focus:bg-hover disabled:opacity-40 ${
                    it.danger ? "text-bad hover:bg-bad-soft focus:bg-bad-soft" : "text-fg hover:bg-hover"
                  }`}
                >
                  {it.icon && <span className="flex h-4 w-4 items-center justify-center text-current opacity-80">{it.icon}</span>}
                  {it.label}
                </button>
              );
            })}
          </div>,
          document.body,
        )}
    </>
  );
}
