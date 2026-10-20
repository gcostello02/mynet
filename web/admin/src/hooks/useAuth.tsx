import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { ApiError, api } from "../api/client";

type AuthState = {
  checked: boolean;
  authenticated: boolean;
  expiresAt: string | null;
  refresh: () => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [checked, setChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [expiresAt, setExpiresAt] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      await api.status();
      setAuthenticated(true);
    } catch (e) {
      setAuthenticated(false);
      setExpiresAt(null);
      if (e instanceof ApiError && e.status === 401) {
        // session expired or coordinator restarted
      }
    } finally {
      setChecked(true);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.login(username, password);
    setExpiresAt(res.expires_at);
    setAuthenticated(true);
    setChecked(true);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } finally {
      setAuthenticated(false);
      setExpiresAt(null);
    }
  }, []);

  const value = useMemo(
    () => ({ checked, authenticated, expiresAt, refresh, login, logout }),
    [checked, authenticated, expiresAt, refresh, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside provider");
  return ctx;
}
