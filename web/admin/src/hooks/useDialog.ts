import { useEffect, useRef, type RefObject } from "react";

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

let scrollLocks = 0;

/**
 * Shared dialog behaviour: moves focus in, traps Tab, closes on Escape,
 * locks background scroll and restores focus to the opener on close.
 */
export function useDialog(open: boolean, panelRef: RefObject<HTMLElement | null>, onClose: () => void) {
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    if (!open) return;
    const opener = document.activeElement as HTMLElement | null;
    const panel = panelRef.current;
    if (!panel) return;

    const items = () => Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE));
    const initial = panel.querySelector<HTMLElement>("[data-autofocus]") ?? items()[0] ?? panel;
    initial.focus({ preventScroll: true });

    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        closeRef.current();
        return;
      }
      if (e.key !== "Tab") return;
      const list = items();
      if (list.length === 0) {
        e.preventDefault();
        panel.focus();
        return;
      }
      const first = list[0];
      const last = list[list.length - 1];
      const active = document.activeElement;
      if (e.shiftKey && (active === first || !panel.contains(active))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (active === last || !panel.contains(active))) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey, true);

    scrollLocks++;
    document.body.style.overflow = "hidden";

    return () => {
      document.removeEventListener("keydown", onKey, true);
      scrollLocks--;
      if (scrollLocks <= 0) {
        scrollLocks = 0;
        document.body.style.overflow = "";
      }
      if (opener && document.contains(opener)) opener.focus({ preventScroll: true });
    };
  }, [open, panelRef]);
}
