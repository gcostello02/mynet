import { NavLink, Outlet, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  MonitorSmartphone,
  Ticket,
  Settings,
  LogOut,
  Sun,
  Moon,
  Menu,
  X,
  Network,
} from "lucide-react";
import { useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { useTheme } from "../hooks/useTheme";
import { Button } from "../components/ui/Button";

const nav = [
  { to: "/", label: "Overview", icon: LayoutDashboard, end: true },
  { to: "/devices", label: "Devices", icon: MonitorSmartphone },
  { to: "/invitations", label: "Invitations", icon: Ticket },
  { to: "/settings", label: "Settings", icon: Settings },
];

function NavItem({
  to,
  label,
  icon: Icon,
  end,
  onNavigate,
}: {
  to: string;
  label: string;
  icon: typeof LayoutDashboard;
  end?: boolean;
  onNavigate?: () => void;
}) {
  return (
    <NavLink
      to={to}
      end={end}
      onClick={onNavigate}
      className={({ isActive }) =>
        `flex items-center gap-2.5 rounded-[var(--radius-md)] px-3 py-2 text-sm font-medium transition-colors ${
          isActive
            ? "bg-[var(--color-surface-overlay)] text-[var(--color-fg)]"
            : "text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-overlay)]/60 hover:text-[var(--color-fg)]"
        }`
      }
    >
      <Icon className="h-4 w-4 shrink-0 opacity-80" aria-hidden />
      {label}
    </NavLink>
  );
}

export function AppShell() {
  const { logout, expiresAt } = useAuth();
  const { theme, toggle } = useTheme();
  const [mobileOpen, setMobileOpen] = useState(false);
  const location = useLocation();

  const closeMobile = () => setMobileOpen(false);

  return (
    <div className="flex min-h-screen bg-[var(--color-surface)]">
      <aside
        className="hidden w-56 shrink-0 flex-col border-r border-[var(--color-border)] bg-[var(--color-surface-raised)] md:flex"
      >
        <div className="flex h-14 items-center gap-2 border-b border-[var(--color-border)] px-4">
          <div className="flex h-8 w-8 items-center justify-center rounded-[var(--radius-md)] bg-[var(--color-accent)]/15 text-[var(--color-accent)]">
            <Network className="h-4 w-4" aria-hidden />
          </div>
          <span className="text-sm font-semibold tracking-tight">MyNet</span>
        </div>
        <nav className="flex flex-1 flex-col gap-0.5 p-3" aria-label="Main">
          {nav.map((item) => (
            <NavItem key={item.to} {...item} />
          ))}
        </nav>
        <div className="border-t border-[var(--color-border)] p-3 text-xs text-[var(--color-fg-subtle)]">
          {expiresAt && (
            <p className="mb-2 px-1" title="Administrator session expiry">
              Session until {new Date(expiresAt).toLocaleTimeString()}
            </p>
          )}
          <div className="flex gap-1">
            <Button variant="ghost" size="sm" className="flex-1" onClick={toggle} aria-label="Toggle theme">
              {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="flex-1"
              onClick={() => void logout()}
            >
              <LogOut className="h-4 w-4" />
              <span className="sr-only sm:not-sr-only">Log out</span>
            </Button>
          </div>
        </div>
      </aside>

      {mobileOpen && (
        <div className="fixed inset-0 z-40 md:hidden" role="presentation">
          <button
            type="button"
            className="absolute inset-0 bg-black/50"
            aria-label="Close menu"
            onClick={closeMobile}
          />
          <aside
            className="absolute left-0 top-0 flex h-full w-64 flex-col border-r border-[var(--color-border)] bg-[var(--color-surface-raised)] shadow-xl"
          >
            <div className="flex h-14 items-center justify-between border-b border-[var(--color-border)] px-4">
              <span className="font-semibold">MyNet</span>
              <Button variant="ghost" size="sm" onClick={closeMobile} aria-label="Close">
                <X className="h-4 w-4" />
              </Button>
            </div>
            <nav className="flex flex-col gap-0.5 p-3" aria-label="Main mobile">
              {nav.map((item) => (
                <NavItem key={item.to} {...item} onNavigate={closeMobile} />
              ))}
            </nav>
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center gap-3 border-b border-[var(--color-border)] bg-[var(--color-surface-raised)] px-4 md:hidden">
          <Button variant="ghost" size="sm" onClick={() => setMobileOpen(true)} aria-label="Open menu">
            <Menu className="h-5 w-5" />
          </Button>
          <span className="text-sm font-medium capitalize">
            {nav.find((n) => (n.end ? location.pathname === n.to : location.pathname.startsWith(n.to)))
              ?.label ?? "MyNet"}
          </span>
        </header>
        <main className="flex-1 p-4 sm:p-6 lg:p-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
