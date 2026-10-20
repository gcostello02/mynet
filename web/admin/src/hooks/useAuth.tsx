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
import { ApiError, api, setUnauthorizedHandler } from "../api/client";

type AuthState = {
  checked: boolean;
  authenticated: boolean;
  sessionExpired: boolean;
  expiresAt: string | null;
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

  // Any 401 after a session existed means it expired or the coordinator restarted.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (hadSession.current) setSessionExpired(true);
      hadSession.current = false;
      setAuthenticated(false);
      setExpiresAt(null);
    });
    return () => setUnauthorizedHandler(null);
  }, []);

  useEffect(() => {
    api
      .status()
      .then(() => {
        hadSession.current = true;
        setAuthenticated(true);
      })
      .catch((e: unknown) => {
        if (!(e instanceof ApiError)) setAuthenticated(false);
      })
      .finally(() => setChecked(true));
  }, []);

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
      hadSession.current = false;
      setAuthenticated(false);
      setExpiresAt(null);
      setSessionExpired(false);
    }
  }, []);

  const value = useMemo(
    () => ({ checked, authenticated, sessionExpired, expiresAt, login, logout }),
    [checked, authenticated, sessionExpired, expiresAt, login, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside provider");
  return ctx;
}
