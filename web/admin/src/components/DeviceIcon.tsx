import { HardDrive, Laptop, Monitor, Server, Smartphone, type LucideIcon } from "lucide-react";

/** Best-effort icon from the device name only; purely cosmetic. */
export function deviceIconFor(name: string): LucideIcon {
  const n = name.toLowerCase();
  if (/(phone|pixel|iphone|android|galaxy|ipad|tablet)/.test(n)) return Smartphone;
  if (/(nas|storage|disk|backup)/.test(n)) return HardDrive;
  if (/(server|srv|pi\b|raspberry|vps|node|box|host)/.test(n)) return Server;
  if (/(mac|book|laptop|thinkpad|xps)/.test(n)) return Laptop;
  return Monitor;
}

export function DeviceIcon({ name, muted = false }: { name: string; muted?: boolean }) {
  const Icon = deviceIconFor(name);
  return (
    <span
      className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-line-strong bg-raised ${
        muted ? "text-fg-subtle" : "text-fg-muted"
      }`}
      aria-hidden
    >
      <Icon className="h-3.5 w-3.5" />
    </span>
  );
}
