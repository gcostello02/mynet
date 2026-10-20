import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { ApiError, api } from "../api/client";

type AuthState = {
  checked: boolean;
  authenticated: boolean;
  sessionExpired: boolean;
  expiresAt: string | null;
  refresh: () => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [checked, setChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [sessionExpired, setSessionExpired] = useState(false);
  const [expiresAt, setExpiresAt] = useState<string | null>(null);
  const hadSession = useRef(false);

  useEffect(() => {
    if (authenticated) hadSession.current = true;
  }, [authenticated]);

  const refresh = useCallback(async () => {
    try {
      await api.status();
      setAuthenticated(true);
      setSessionExpired(false);
    } catch (e) {
      setAuthenticated(false);
      setExpiresAt(null);
      if (e instanceof ApiError && e.status === 401 && hadSession.current) {
        setSessionExpired(true);
        hadSession.current = false;
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
    setSessionExpired(false);
    hadSession.current = true;
    setChecked(true);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } finally {
      setAuthenticated(false);
      setExpiresAt(null);
      setSessionExpired(false);
      hadSession.current = false;
    }
  }, []);

  const value = useMemo(
    () => ({
      checked,
      authenticated,
      sessionExpired,
      expiresAt,
      refresh,
      login,
      logout,
    }),
    [checked, authenticated, sessionExpired, expiresAt, refresh, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside provider");
  return ctx;
}
