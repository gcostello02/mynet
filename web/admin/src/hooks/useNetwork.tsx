import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, type AdminStatus, type NetworkPublic } from "../api/client";

type NetworkState = {
  status: AdminStatus | null;
  network: NetworkPublic | null;
  healthy: boolean | null;
  loaded: boolean;
  refresh: () => Promise<void>;
};

const NetworkContext = createContext<NetworkState | null>(null);

/** Coordinator-wide data shared by the top bar and pages. Polls while the tab is visible. */
export function NetworkProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AdminStatus | null>(null);
  const [network, setNetwork] = useState<NetworkPublic | null>(null);
  const [healthy, setHealthy] = useState<boolean | null>(null);
  const [loaded, setLoaded] = useState(false);

  const refresh = useCallback(async () => {
    const [st, np, hp] = await Promise.allSettled([
      api.status(),
      api.networkPublic(),
      api.health().then((r) => r.ok),
    ]);
    if (st.status === "fulfilled") setStatus(st.value);
    if (np.status === "fulfilled") setNetwork(np.value);
    setHealthy(hp.status === "fulfilled" ? hp.value : false);
    setLoaded(true);
  }, []);

  useEffect(() => {
    void refresh();
    const t = window.setInterval(() => {
      if (document.visibilityState === "visible") void refresh();
    }, 30000);
    return () => window.clearInterval(t);
  }, [refresh]);

  const value = useMemo(() => ({ status, network, healthy, loaded, refresh }), [status, network, healthy, loaded, refresh]);
  return <NetworkContext.Provider value={value}>{children}</NetworkContext.Provider>;
}

export function useNetwork() {
  const ctx = useContext(NetworkContext);
  if (!ctx) throw new Error("useNetwork outside provider");
  return ctx;
}
