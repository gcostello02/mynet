import { readCSRFCookie } from "../lib/csrf";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function parseError(res: Response): Promise<string> {
  try {
    const j = (await res.json()) as { error?: string };
    return j.error ?? res.statusText;
  } catch {
    return res.statusText;
  }
}

let unauthorizedHandler: (() => void) | null = null;

/** AuthProvider registers this so any 401 on an authenticated call forces a clean re-login. */
export function setUnauthorizedHandler(fn: (() => void) | null) {
  unauthorizedHandler = fn;
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit & { json?: unknown } = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.json !== undefined) {
    headers.set("Content-Type", "application/json");
  }
  const method = (init.method ?? "GET").toUpperCase();
  if (method !== "GET" && method !== "HEAD") {
    const csrf = readCSRFCookie();
    if (csrf) headers.set("X-Mynet-CSRF", csrf);
  }
  const res = await fetch(path, {
    ...init,
    method,
    headers,
    credentials: "include",
    body: init.json !== undefined ? JSON.stringify(init.json) : init.body,
  });
  if (!res.ok) {
    if (res.status === 401 && path !== "/v1/admin/login") unauthorizedHandler?.();
    throw new ApiError(res.status, await parseError(res));
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

export type SetupStatus = {
  initialized: boolean;
  requires_bootstrap_proof?: boolean;
  bootstrap_active?: boolean;
  bootstrap_expires_at?: string;
};

export type NetworkPublic = {
  network_id: string;
  coordinator_public_key: string;
  coordinator_fingerprint: string;
  public_server_url: string;
  network_display_name: string;
};

export type AdminStatus = {
  setup_complete: boolean;
  device_count: number;
  active_device_count: number;
  revoked_device_count: number;
  signaling_online_count: number;
  legacy_enrollment: boolean;
  coordinator_fingerprint: string;
  server_version: string;
};

export type Device = {
  device_id: string;
  name: string;
  vpn_ipv4: string;
  revoked: boolean;
  signaling_online: boolean;
  enrolled_at: string;
  auth_epoch: number;
};

export type Invitation = {
  id: string;
  label: string;
  expires_at: string;
  max_uses: number;
  uses: number;
  revoked: boolean;
  created_at: string;
};

export const api = {
  health: () => fetch("/healthz", { credentials: "include" }),
  setupStatus: () => apiFetch<SetupStatus>("/v1/setup/status"),
  setup: (
    body: {
      username: string;
      password: string;
      network_display_name: string;
      public_server_url: string;
    },
    bootstrapToken: string,
  ) =>
    apiFetch<{ coordinator_fingerprint: string; admin_id: string }>("/v1/admin/setup", {
      method: "POST",
      json: body,
      headers: { "X-Mynet-Bootstrap-Token": bootstrapToken },
    }),
  networkPublic: () => apiFetch<NetworkPublic>("/v1/network/public"),
  login: (username: string, password: string) =>
    apiFetch<{ admin_id: string; expires_at: string }>("/v1/admin/login", {
      method: "POST",
      json: { username, password },
    }),
  logout: () => apiFetch<void>("/v1/admin/logout", { method: "POST" }),
  status: () => apiFetch<AdminStatus>("/v1/admin/status"),
  devices: () => apiFetch<Device[]>("/v1/admin/devices"),
  revokeDevice: (id: string) =>
    apiFetch<void>(`/v1/admin/devices/${encodeURIComponent(id)}/revoke`, {
      method: "POST",
      json: {},
    }),
  invitations: () => apiFetch<Invitation[]>("/v1/admin/invitations"),
  createInvitation: (body: {
    label: string;
    expires_in_seconds: number;
    max_uses: number;
  }) =>
    apiFetch<{
      id: string;
      enrollment_token: string;
      coordinator_fingerprint: string;
      public_server_url: string;
    }>("/v1/admin/invitations", { method: "POST", json: body }),
  deleteInvitation: (id: string) =>
    apiFetch<void>(`/v1/admin/invitations/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
};
