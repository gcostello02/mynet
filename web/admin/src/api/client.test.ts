import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiFetch, setUnauthorizedHandler } from "./client";

describe("apiFetch", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    document.cookie = "";
  });

  it("sends CSRF header on POST when cookie present", async () => {
    document.cookie = "mynet_csrf=tok; path=/";
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      text: async () => "",
    });
    vi.stubGlobal("fetch", fetchMock);
    await apiFetch("/v1/admin/logout", { method: "POST" });
    const headers = fetchMock.mock.calls[0][1].headers as Headers;
    expect(headers.get("X-Mynet-CSRF")).toBe("tok");
  });

  it("does not treat wrong current password on change as global session expiry", async () => {
    const onUnauthorized = vi.fn();
    setUnauthorizedHandler(onUnauthorized);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 403,
        json: async () => ({ error: "invalid current password" }),
      }),
    );
    await expect(
      apiFetch("/v1/admin/password", { method: "POST", json: { current_password: "x", new_password: "y" } }),
    ).rejects.toMatchObject({ status: 403 });
    expect(onUnauthorized).not.toHaveBeenCalled();
    setUnauthorizedHandler(null);
  });

  it("throws ApiError with status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: async () => ({ error: "admin authentication required" }),
      }),
    );
    await expect(apiFetch("/v1/admin/status")).rejects.toEqual(
      new ApiError(401, "admin authentication required"),
    );
  });
});
