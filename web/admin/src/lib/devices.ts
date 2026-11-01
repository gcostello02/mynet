import type { Device } from "../api/client";

export function sortDevicesByEnrolled(devs: Device[]): Device[] {
  return [...devs].sort((a, b) => new Date(b.enrolled_at).getTime() - new Date(a.enrolled_at).getTime());
}

export type DeviceCounts = { total: number; active: number; revoked: number; online: number };

export function countDevices(devs: Device[]): DeviceCounts {
  let active = 0;
  let revoked = 0;
  let online = 0;
  for (const d of devs) {
    if (d.revoked) revoked++;
    else {
      active++;
      if (d.signaling_online) online++;
    }
  }
  return { total: devs.length, active, revoked, online };
}

/** Accurate, scoped wording: signaling presence is not tunnel state. */
export function signalingLabel(d: Device): { text: string; tone: "ok" | "neutral" | "bad"; hollow?: boolean } {
  if (d.revoked) return { text: "Not applicable", tone: "neutral", hollow: true };
  return d.signaling_online
    ? { text: "Signaling online", tone: "ok" }
    : { text: "Signaling offline", tone: "neutral", hollow: true };
}
