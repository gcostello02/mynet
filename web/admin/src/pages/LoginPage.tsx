import { Eye, EyeOff, Lock, ServerCog, ShieldCheck, Ticket } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { ApiError } from "../api/client";
import { Logo } from "../components/brand/Logo";
import { Button } from "../components/ui/Button";
import { Input } from "../components/ui/Input";
import { useAuth } from "../hooks/useAuth";

const points = [
  { icon: ServerCog, title: "Self-hosted", body: "Your coordinator, your database, your keys. Nothing phones home." },
  { icon: Ticket, title: "Invitation-only enrollment", body: "Devices join with one-time secrets you create and can revoke." },
  { icon: ShieldCheck, title: "Private by design", body: "No third-party scripts, fonts, or analytics load in this console." },
];

export function LoginPage() {
  const { login, authenticated, checked, sessionExpired } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [show, setShow] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const expired = sessionExpired || params.get("expired") === "1";
  const passwordChanged = params.get("password_changed") === "1";

  if (checked && authenticated) return <Navigate to="/" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await login(username, password);
      navigate("/", { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(
          err.status === 401
            ? "That username and password combination wasn’t recognized."
            : err.status === 429
              ? "Too many attempts. Wait a minute before trying again."
              : err.message,
        );
      } else {
        setError("Can’t reach the coordinator. Check your connection and try again.");
      }
      setPassword("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid min-h-screen lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
      <aside className="bg-dots relative hidden flex-col justify-between overflow-hidden border-r border-line bg-panel p-12 lg:flex">
        <Logo size="lg" />
        <div className="max-w-md">
          <h2 className="text-[28px] font-semibold leading-tight tracking-tight">
            Your mesh, <span className="text-accent">under your control.</span>
          </h2>
          <p className="mt-3 text-[15px] leading-relaxed text-fg-muted">
            MyNet is a fully self-hosted WireGuard mesh VPN. This console manages enrollment and access for your
            network, directly from your own coordinator.
          </p>
          <ul className="mt-10 space-y-5">
            {points.map(({ icon: Icon, title, body }) => (
              <li key={title} className="flex gap-3.5">
                <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-line-strong bg-raised text-accent">
                  <Icon className="h-4 w-4" aria-hidden />
                </span>
                <span>
                  <span className="block text-[13px] font-medium">{title}</span>
                  <span className="mt-0.5 block text-[13px] leading-relaxed text-fg-muted">{body}</span>
                </span>
              </li>
            ))}
          </ul>
        </div>
        <p className="font-mono text-xs text-fg-subtle">{window.location.host}</p>
      </aside>

      <main className="flex flex-col items-center justify-center px-5 py-12">
        <div className="mb-10 lg:hidden">
          <Logo size="lg" />
        </div>
        <div className="w-full max-w-[360px]">
          <h1 className="text-xl font-semibold tracking-tight">Sign in</h1>
          <p className="mt-1.5 text-[13px] text-fg-muted">Use your coordinator administrator account.</p>

          {passwordChanged && !error && (
            <p role="status" className="mt-5 rounded-md border border-ok/30 bg-ok-soft px-3 py-2 text-[13px] text-ok">
              Password updated. Sign in with your new password.
            </p>
          )}
          {expired && !passwordChanged && !error && (
            <p role="status" className="mt-5 rounded-md border border-warn/30 bg-warn-soft px-3 py-2 text-[13px] text-warn">
              Your session ended — it expired or the coordinator restarted. Sign in again to continue.
            </p>
          )}
          {error && (
            <p role="alert" className="mt-5 rounded-md border border-bad/30 bg-bad-soft px-3 py-2 text-[13px] text-bad">
              {error}
            </p>
          )}

          <form onSubmit={onSubmit} className="mt-6 space-y-4">
            <Input
              label="Username"
              autoComplete="username"
              autoFocus
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              className="h-9"
            />
            <div>
              <div className="relative">
                <Input
                  label="Password"
                  type={show ? "text" : "password"}
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  className="h-9 pr-9"
                />
                <button
                  type="button"
                  aria-label={show ? "Hide password" : "Show password"}
                  aria-pressed={show}
                  onClick={() => setShow((s) => !s)}
                  className="absolute bottom-0 right-0 flex h-9 w-9 items-center justify-center text-fg-subtle hover:text-fg"
                >
                  {show ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>
            <Button variant="primary" loading={busy} className="h-9 w-full text-sm" {...{ type: "submit" }}>
              {busy ? "Signing in…" : "Sign in"}
            </Button>
          </form>

          <p className="mt-8 flex items-center gap-2 text-xs text-fg-subtle">
            <Lock className="h-3.5 w-3.5 shrink-0" aria-hidden />
            Sessions use secure, HttpOnly cookies. Nothing is stored in your browser.
          </p>
        </div>
      </main>
    </div>
  );
}
