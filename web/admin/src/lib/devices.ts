import type { Device } from "../api/client";

export function sortDevicesByEnrolled(devs: Device[]): Device[] {
  return [...devs].sort(
    (a, b) => new Date(b.enrolled_at).getTime() - new Date(a.enrolled_at).getTime(),
  );
}

export function formatEnrolled(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      dateStyle: "medium",
      timeStyle: "short",
    });
  } catch {
    return iso;
  }
}
