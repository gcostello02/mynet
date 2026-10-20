import { NavLink, Outlet } from "react-router-dom";
import {
  LayoutDashboard,
  MonitorSmartphone,
  Ticket,
  Settings,
  LogOut,
  Sun,
  Moon,
} from "lucide-react";
import { useAuth } from "../hooks/useAuth";
import { useTheme } from "../hooks/useTheme";

const nav = [
  { to: "/", label: "Overview", icon: LayoutDashboard, end: true },
  { to: "/devices", label: "Devices", icon: MonitorSmartphone },
  { to: "/invitations", label: "Invitations", icon: Ticket },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function AppShell() {
  const { logout, expiresAt } = useAuth();
  const { theme, toggle } = useTheme();

  return (
    <div className="flex min-h-screen bg-zinc-950 text-zinc-100 light:bg-zinc-50 light:text-zinc-900">
      <aside className="hidden w-56 shrink-0 border-r border-zinc-800 bg-zinc-950/80 p-4 md:block light:border-zinc-200 light:bg-white">
        <div className="mb-8 text-sm font-semibold tracking-wide text-zinc-300 light:text-zinc-700">
          MyNet
        </div>
        <nav className="space-y-1">
          {nav.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                `flex items-center gap-2 rounded-lg px-3 py-2 text-sm ${
                  isActive
                    ? "bg-zinc-800 text-white light:bg-zinc-100 light:text-zinc-900"
                    : "text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100 light:hover:bg-zinc-50"
                }`
              }
            >
              <Icon className="h-4 w-4" aria-hidden />
              {label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center justify-between border-b border-zinc-800 px-4 py-3 light:border-zinc-200">
          <nav className="flex gap-2 md:hidden">
            {nav.map(({ to, label }) => (
              <NavLink
                key={to}
                to={to}
                className="rounded-lg px-2 py-1 text-xs text-zinc-400 hover:text-zinc-100"
              >
                {label}
              </NavLink>
            ))}
          </nav>
          <div className="ml-auto flex items-center gap-2 text-xs text-zinc-500">
            {expiresAt && (
              <span title="Session expires">Session until {new Date(expiresAt).toLocaleString()}</span>
            )}
            <button
              type="button"
              className="rounded-lg p-2 hover:bg-zinc-800 light:hover:bg-zinc-100"
              onClick={toggle}
              aria-label="Toggle theme"
            >
              {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-1 rounded-lg px-2 py-1 hover:bg-zinc-800 light:hover:bg-zinc-100"
              onClick={() => void logout()}
            >
              <LogOut className="h-4 w-4" />
              Log out
            </button>
          </div>
        </header>
        <main className="flex-1 p-4 md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
