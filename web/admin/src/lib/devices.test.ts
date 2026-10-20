import { describe, expect, it } from "vitest";
import { sortDevicesByEnrolled } from "./devices";
import type { Device } from "../api/client";

const base = (name: string, enrolled_at: string): Device => ({
  device_id: name,
  name,
  vpn_ipv4: "",
  revoked: false,
  signaling_online: false,
  enrolled_at,
  auth_epoch: 0,
});

describe("sortDevicesByEnrolled", () => {
  it("sorts newest first", () => {
    const sorted = sortDevicesByEnrolled([
      base("a", "2024-01-01T00:00:00Z"),
      base("b", "2025-01-01T00:00:00Z"),
    ]);
    expect(sorted[0].name).toBe("b");
  });
});
