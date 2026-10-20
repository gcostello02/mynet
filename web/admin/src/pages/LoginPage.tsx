import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { ApiError } from "../api/client";
import { useAuth } from "../hooks/useAuth";

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await login(username, password);
      navigate("/", { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.status === 401 ? "Invalid username or password." : err.message);
      } else {
        setError("Could not reach coordinator.");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-950 p-4 light:bg-zinc-100">
      <form
        onSubmit={onSubmit}
        className="w-full max-w-sm rounded-xl border border-zinc-800 bg-zinc-900 p-6 shadow-lg light:border-zinc-200 light:bg-white"
      >
        <h1 className="text-xl font-semibold">MyNet Admin</h1>
        <p className="mt-1 text-sm text-zinc-400">Sign in with your coordinator administrator account.</p>
        {error && (
          <p className="mt-4 rounded-lg bg-red-500/10 px-3 py-2 text-sm text-red-300" role="alert">
            {error}
          </p>
        )}
        <label className="mt-4 block text-sm">
          <span className="text-zinc-400">Username</span>
          <input
            className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-zinc-100 light:border-zinc-300 light:bg-white light:text-zinc-900"
            autoComplete="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
          />
        </label>
        <label className="mt-3 block text-sm">
          <span className="text-zinc-400">Password</span>
          <input
            type="password"
            className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-zinc-100 light:border-zinc-300 light:bg-white light:text-zinc-900"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </label>
        <button
          type="submit"
          disabled={busy}
          className="mt-5 w-full rounded-lg bg-sky-600 py-2 text-sm font-medium text-white hover:bg-sky-500 disabled:opacity-50"
        >
          {busy ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  );
}
