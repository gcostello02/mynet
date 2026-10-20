import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { MembershipMap } from "../MembershipMap";
import { ConfirmDialog } from "./ConfirmDialog";
import { Modal } from "./Modal";

function Harness() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>opener</button>
      <Modal open={open} onClose={() => setOpen(false)} title="Hello">
        <button>first</button>
        <button>last</button>
      </Modal>
    </>
  );
}

describe("Modal", () => {
  it("traps focus, closes on Escape, and restores focus to the opener", () => {
    render(<Harness />);
    const opener = screen.getByText("opener");
    opener.focus();
    fireEvent.click(opener);
    expect(screen.getByRole("dialog", { name: "Hello" })).toBeInTheDocument();

    // Tab from the last control wraps within the dialog instead of escaping it.
    const dialog = screen.getByRole("dialog");
    screen.getByText("last").focus();
    fireEvent.keyDown(document.activeElement ?? document.body, { key: "Tab" });
    expect(dialog.contains(document.activeElement)).toBe(true);
    expect(opener).not.toHaveFocus();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });
});

describe("ConfirmDialog", () => {
  it("requires typed text before confirming", () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        open
        title="Revoke?"
        description="desc"
        confirmLabel="Revoke device"
        requireText="laptop"
        onConfirm={onConfirm}
        onCancel={() => {}}
      />,
    );
    const btn = screen.getByRole("button", { name: "Revoke device" });
    expect(btn).toBeDisabled();
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "laptop" } });
    expect(btn).toBeEnabled();
    fireEvent.click(btn);
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});

describe("MembershipMap", () => {
  it("describes enrollment only and never claims tunnels", () => {
    const { container } = render(
      <MemoryRouter>
        <MembershipMap
          devices={[
            { device_id: "a", name: "alpha", vpn_ipv4: "10.0.0.1", revoked: false, signaling_online: true, enrolled_at: "2026-01-01T00:00:00Z", auth_epoch: 0 },
          ]}
        />
      </MemoryRouter>,
    );
    const text = container.textContent ?? "";
    expect(text).toMatch(/not live tunnels/i);
    expect(text).not.toMatch(/\bconnected\b/i);
    expect(screen.getByRole("img").getAttribute("aria-label")).toMatch(/enrolled/);
  });
});
