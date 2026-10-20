import {
  LayoutDashboard,
  LogOut,
  Menu as MenuIcon,
  Monitor,
  Moon,
  MonitorSmartphone,
  Settings,
  Sun,
  Ticket,
  X,
  ChevronDown,
} from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { Logo } from "../components/brand/Logo";
import { StatusDot } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { useCopy } from "../components/ui/Copy";
import { Menu } from "../components/ui/Menu";
import { useAuth } from "../hooks/useAuth";
import { NetworkProvider, useNetwork } from "../hooks/useNetwork";
import { useDialog } from "../hooks/useDialog";
import { useTheme } from "../hooks/useTheme";
import { shortFingerprint } from "../lib/time";

const nav = [
  { to: "/", label: "Overview", icon: LayoutDashboard, end: true },
  { to: "/devices", label: "Devices", icon: MonitorSmartphone },
  { to: "/invitations", label: "Invitations", icon: Ticket },
  { to: "/settings", label: "Settings", icon: Settings },
];

function NavList({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <nav aria-label="Primary" className="flex flex-col gap-0.5">
      {nav.map(({ to, label, icon: Icon, end }) => (
        <NavLink
          key={to}
          to={to}
          end={end}
          onClick={onNavigate}
          className={({ isActive }) =>
            `group relative flex h-8 items-center gap-2.5 rounded-md px-2.5 text-[13px] font-medium transition-colors ${
              isActive ? "bg-hover text-fg" : "text-fg-muted hover:bg-hover/60 hover:text-fg"
            }`
          }
        >
          {({ isActive }) => (
            <>
              {isActive && <span className="absolute -left-3 h-4 w-0.5 rounded-full bg-accent" aria-hidden />}
              <Icon className={`h-4 w-4 ${isActive ? "text-accent" : "text-fg-subtle group-hover:text-fg-muted"}`} aria-hidden />
              {label}
            </>
          )}
        </NavLink>
      ))}
    </nav>
  );
}

function SidebarBody({ onNavigate }: { onNavigate?: () => void }) {
  const { network, status } = useNetwork();
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-12 items-center px-4">
        <Logo />
      </div>
      <div className="px-3 pb-3">
        <div className="rounded-md border border-line bg-canvas/60 px-2.5 py-2">
          <div className="text-[10px] font-medium uppercase tracking-wider text-fg-subtle">Network</div>
          <div className="mt-0.5 truncate text-[13px] font-medium" title={network?.network_display_name}>
            {network?.network_display_name || "Unnamed network"}
          </div>
        </div>
      </div>
      <div className="flex-1 px-3">
        <NavList onNavigate={onNavigate} />
      </div>
      <div className="space-y-1 border-t border-line px-4 py-3 text-[11px] text-fg-subtle">
        <div className="flex items-center justify-between">
          <span>Coordinator</span>
          <span className="font-mono">{status?.server_version ?? "—"}</span>
        </div>
        <div className="truncate font-mono text-fg-muted" title={window.location.host}>
          {window.location.host}
        </div>
      </div>
    </div>
  );
}

function MobileNav({ open, onClose }: { open: boolean; onClose: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useDialog(open, ref, onClose);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-40 lg:hidden">
      <div className="anim-fade absolute inset-0 bg-black/60" aria-hidden onClick={onClose} />
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-label="Navigation"
        tabIndex={-1}
        className="anim-slide absolute inset-y-0 left-0 w-64 border-r border-line-strong bg-panel shadow-pop outline-none"
      >
        <Button variant="ghost" size="icon" aria-label="Close navigation" onClick={onClose} className="absolute right-2 top-2.5">
          <X className="h-4 w-4" />
        </Button>
        <SidebarBody onNavigate={onClose} />
      </div>
    </div>
  );
}

function HealthPill() {
  const { healthy, loaded } = useNetwork();
  if (!loaded || healthy === null) {
    return <span className="hidden h-6 w-28 animate-pulse rounded-full bg-hover sm:block" aria-hidden />;
  }
  return (
    <span
      className={`hidden h-6 items-center gap-1.5 rounded-full border px-2.5 text-xs font-medium sm:inline-flex ${
        healthy ? "border-ok/25 bg-ok-soft text-ok" : "border-bad/25 bg-bad-soft text-bad"
      }`}
      role="status"
    >
      <StatusDot tone={healthy ? "ok" : "bad"} />
      {healthy ? "Coordinator healthy" : "Coordinator unreachable"}
    </span>
  );
}

function FingerprintChip() {
  const { status } = useNetwork();
  const { copied, copy } = useCopy();
  if (!status?.coordinator_fingerprint) return null;
  return (
    <button
      type="button"
      onClick={() => void copy(status.coordinator_fingerprint)}
      title="Coordinator fingerprint — click to copy the full value"
      className="hidden h-6 items-center gap-1.5 rounded-md border border-line px-2 font-mono text-[11px] text-fg-muted transition-colors hover:border-line-strong hover:text-fg xl:inline-flex"
    >
      <span className="font-sans text-[10px] uppercase tracking-wider text-fg-subtle">FP</span>
      {copied ? "Copied" : shortFingerprint(status.coordinator_fingerprint)}
    </button>
  );
}

function UserMenu() {
  const { logout, expiresAt } = useAuth();
  const { pref, setPref } = useTheme();
  return (
    <Menu
      label="Account menu"
      triggerClassName="flex h-8 items-center gap-1.5 rounded-md pl-1 pr-1.5 text-fg-muted transition-colors hover:bg-hover hover:text-fg"
      trigger={
        <>
          <span className="flex h-6 w-6 items-center justify-center rounded-full border border-line-strong bg-raised text-[11px] font-semibold text-fg">
            A
          </span>
          <ChevronDown className="h-3.5 w-3.5" aria-hidden />
        </>
      }
      items={[
        { type: "label", label: "Signed in as administrator" },
        ...(expiresAt
          ? [{ type: "label" as const, label: `Session until ${new Date(expiresAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}` }]
          : []),
        { type: "separator" },
        { type: "label", label: "Appearance" },
        { label: pref === "dark" ? "Dark ✓" : "Dark", icon: <Moon className="h-4 w-4" />, onSelect: () => setPref("dark") },
        { label: pref === "light" ? "Light ✓" : "Light", icon: <Sun className="h-4 w-4" />, onSelect: () => setPref("light") },
        { label: pref === "system" ? "System ✓" : "System", icon: <Monitor className="h-4 w-4" />, onSelect: () => setPref("system") },
        { type: "separator" },
        { label: "Sign out", icon: <LogOut className="h-4 w-4" />, onSelect: () => void logout() },
      ]}
    />
  );
}

function TopBar({ onMenu }: { onMenu: () => void }) {
  const { network } = useNetwork();
  const location = useLocation();
  const current = nav.find((n) => (n.end ? location.pathname === n.to : location.pathname.startsWith(n.to)));
  return (
    <header className="sticky top-0 z-30 flex h-12 shrink-0 items-center gap-3 border-b border-line bg-canvas/90 px-4 backdrop-blur-sm lg:px-6">
      <Button variant="ghost" size="icon" aria-label="Open navigation" onClick={onMenu} className="lg:hidden">
        <MenuIcon className="h-4 w-4" />
      </Button>
      <nav aria-label="Breadcrumb" className="flex min-w-0 items-center gap-1.5 text-[13px]">
        <span className="hidden truncate text-fg-subtle sm:inline">{network?.network_display_name || "Network"}</span>
        <span className="hidden text-fg-subtle sm:inline" aria-hidden>
          /
        </span>
        <span className="truncate font-medium text-fg">{current?.label ?? "MyNet"}</span>
      </nav>
      <div className="ml-auto flex items-center gap-2">
        <FingerprintChip />
        <HealthPill />
        <UserMenu />
      </div>
    </header>
  );
}

function Shell(): ReactNode {
  const [mobile, setMobile] = useState(false);
  const location = useLocation();
  useEffect(() => setMobile(false), [location.pathname]);
  return (
    <div className="min-h-screen">
      <a
        href="#main"
        className="sr-only z-50 rounded-md bg-raised px-3 py-1.5 text-[13px] focus:not-sr-only focus:fixed focus:left-3 focus:top-3"
      >
        Skip to content
      </a>
      <aside className="fixed inset-y-0 left-0 hidden w-56 border-r border-line bg-panel lg:block">
        <SidebarBody />
      </aside>
      <MobileNav open={mobile} onClose={() => setMobile(false)} />
      <div className="flex min-h-screen flex-col lg:pl-56">
        <TopBar onMenu={() => setMobile(true)} />
        <main id="main" tabIndex={-1} className="mx-auto w-full max-w-[1200px] flex-1 px-4 py-6 outline-none lg:px-8 lg:py-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

export function AppShell() {
  return (
    <NetworkProvider>
      <Shell />
    </NetworkProvider>
  );
}
