import { useNavigate } from "react-router-dom";
import type { Device } from "../api/client";

const MAX_NODES = 10;
const W = 720;
const H = 330;
const CX = W / 2;
const CY = 158;
const RX = 285;
const RY = 105;

function clip(s: string, n: number) {
  return s.length > n ? `${s.slice(0, n - 1)}…` : s;
}

/**
 * Network membership around the coordinator.
 * Spokes show enrollment with this coordinator only — they are NOT WireGuard tunnels.
 */
export function MembershipMap({ devices }: { devices: Device[] }) {
  const navigate = useNavigate();
  const shown = devices.slice(0, MAX_NODES);
  const hidden = devices.length - shown.length;
  const n = shown.length + (hidden > 0 ? 1 : 0);

  const pos = (i: number) => {
    const a = -Math.PI / 2 + (i * 2 * Math.PI) / Math.max(n, 1);
    return { x: CX + RX * Math.cos(a), y: CY + RY * Math.sin(a) };
  };

  const open = (id: string) => navigate(`/devices?device=${encodeURIComponent(id)}`);

  return (
    <figure className="m-0">
      <svg
        viewBox={`0 0 ${W} ${H}`}
        role="img"
        aria-label={`Network membership: ${devices.length} enrolled device${devices.length === 1 ? "" : "s"} around this coordinator`}
        className="h-auto w-full"
      >
        <ellipse cx={CX} cy={CY} rx={RX} ry={RY} className="fill-none stroke-line" strokeWidth="1" />

        {shown.map((d, i) => {
          const p = pos(i);
          return (
            <line
              key={`l-${d.device_id}`}
              x1={CX}
              y1={CY}
              x2={p.x}
              y2={p.y}
              className="stroke-line-strong"
              opacity={d.revoked ? 0.45 : 1}
              strokeWidth="1.25"
              strokeDasharray="1.5 5"
              strokeLinecap="round"
            />
          );
        })}

        {/* coordinator */}
        <g>
          <circle cx={CX} cy={CY} r="30" className="fill-raised stroke-line-strong" strokeWidth="1" />
          <circle cx={CX} cy={CY} r="38" className="fill-none stroke-accent" strokeWidth="1" opacity="0.28" />
          <g className="stroke-accent" strokeWidth="1.6" strokeLinecap="round" fill="none">
            <path d={`M${CX} ${CY - 11}L${CX - 10} ${CY + 7}M${CX} ${CY - 11}L${CX + 10} ${CY + 7}M${CX - 10} ${CY + 7}H${CX + 10}`} opacity="0.55" />
          </g>
          <circle cx={CX} cy={CY - 11} r="3" className="fill-accent" />
          <circle cx={CX - 10} cy={CY + 7} r="3" className="fill-accent" />
          <circle cx={CX + 10} cy={CY + 7} r="3" className="fill-accent" />
          <text x={CX} y={CY + 56} textAnchor="middle" className="fill-fg-muted text-[11px] font-medium">
            Coordinator
          </text>
        </g>

        {shown.map((d, i) => {
          const p = pos(i);
          const online = !d.revoked && d.signaling_online;
          const below = p.y >= CY - 4;
          const ly = below ? p.y + 24 : p.y - 28;
          const status = d.revoked ? "revoked" : online ? "signaling online" : "signaling offline";
          return (
            <g
              key={d.device_id}
              role="link"
              tabIndex={0}
              aria-label={`${d.name}, ${status}. Open details`}
              className="cursor-pointer outline-none [&:focus-visible>circle:first-of-type]:stroke-accent"
              onClick={() => open(d.device_id)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  open(d.device_id);
                }
              }}
            >
              <title>{`${d.name} — ${status}`}</title>
              <circle cx={p.x} cy={p.y} r="17" className="fill-panel stroke-transparent" strokeWidth="1.5" />
              <circle cx={p.x} cy={p.y} r="11" className={`${online ? "fill-ok-soft stroke-ok" : d.revoked ? "fill-none stroke-bad" : "fill-none stroke-fg-subtle"}`} strokeWidth="1.5" strokeDasharray={d.revoked ? "2.5 2.5" : undefined} />
              <circle cx={p.x} cy={p.y} r="4" className={online ? "fill-ok" : d.revoked ? "fill-bad" : "fill-fg-subtle"} opacity={d.revoked ? 0.6 : 1} />
              <text x={p.x} y={ly} textAnchor="middle" className={`text-[12px] font-medium ${d.revoked ? "fill-fg-subtle" : "fill-fg"}`} textDecoration={d.revoked ? "line-through" : undefined}>
                {clip(d.name, 16)}
              </text>
              <text x={p.x} y={ly + 14} textAnchor="middle" className="fill-fg-subtle font-mono text-[10px]">
                {d.vpn_ipv4 || "no address"}
              </text>
            </g>
          );
        })}

        {hidden > 0 && (() => {
          const p = pos(shown.length);
          return (
            <g role="link" tabIndex={0} aria-label={`${hidden} more devices. View all`} className="cursor-pointer outline-none" onClick={() => navigate("/devices")} onKeyDown={(e) => e.key === "Enter" && navigate("/devices")}>
              <circle cx={p.x} cy={p.y} r="13" className="fill-raised stroke-line-strong" />
              <text x={p.x} y={p.y + 4} textAnchor="middle" className="fill-fg-muted text-[11px] font-medium">
                +{hidden}
              </text>
            </g>
          );
        })()}
      </svg>

      <figcaption className="flex flex-wrap items-center gap-x-5 gap-y-1.5 border-t border-line px-4 py-2.5 text-xs text-fg-muted">
        <span className="inline-flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-full border border-ok bg-ok-soft" aria-hidden /> Signaling online
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-full border border-fg-subtle" aria-hidden /> Signaling offline
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-full border border-dashed border-bad" aria-hidden /> Revoked
        </span>
        <span className="ml-auto text-fg-subtle">Dotted lines show enrollment, not live tunnels.</span>
      </figcaption>
    </figure>
  );
}
