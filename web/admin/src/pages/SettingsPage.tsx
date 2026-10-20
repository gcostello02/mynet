import type { ReactNode } from "react";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Panel, PanelHeader } from "../components/ui/Card";
import { CopyField } from "../components/ui/Copy";
import { PageHeader } from "../components/ui/PageHeader";
import { Segmented } from "../components/ui/Segmented";
import { Skeleton } from "../components/ui/Skeleton";
import { useAuth } from "../hooks/useAuth";
import { useNetwork } from "../hooks/useNetwork";
import { useTheme, type ThemePref } from "../hooks/useTheme";
import { formatDateTime } from "../lib/time";

function Row({ label, children, note }: { label: string; children: ReactNode; note?: string }) {
  return (
    <div className="grid gap-1.5 px-4 py-3 sm:grid-cols-[180px_1fr] sm:gap-4">
      <dt className="pt-1 text-[13px] text-fg-muted">{label}</dt>
      <dd className="min-w-0 text-[13px]">
        {children}
        {note && <p className="mt-1.5 text-xs text-fg-subtle">{note}</p>}
      </dd>
    </div>
  );
}

function ReadOnly({ children }: { children: ReactNode }) {
  return <span className="inline-flex items-center gap-2">{children}</span>;
}

export function SettingsPage() {
  const { network, status, loaded } = useNetwork();
  const { logout, expiresAt } = useAuth();
  const { pref, setPref } = useTheme();

  return (
    <div className="max-w-3xl">
      <PageHeader
        title="Settings"
        description="Details of this network and coordinator. Values here are read-only unless marked otherwise."
      />

      <div className="space-y-4">
        <Panel>
          <PanelHeader title="Network identity" action={<Badge tone="neutral" dotted={false}>Read-only</Badge>} />
          <dl className="divide-y divide-line">
            <Row label="Network name">
              {loaded ? <span className="font-medium">{network?.network_display_name || "—"}</span> : <Skeleton className="h-4 w-32" />}
            </Row>
            <Row label="Network ID">
              {network ? <CopyField label="Network ID" value={network.network_id} /> : <Skeleton className="h-8" />}
            </Row>
            <Row
              label="Coordinator fingerprint"
              note="Clients compare this value when they join, so they know they’re talking to the right coordinator."
            >
              {network ? (
                <CopyField label="Coordinator fingerprint" value={network.coordinator_fingerprint} />
              ) : (
                <Skeleton className="h-8" />
              )}
            </Row>
          </dl>
        </Panel>

        <Panel>
          <PanelHeader title="Coordinator" action={<Badge tone="neutral" dotted={false}>Read-only</Badge>} />
          <dl className="divide-y divide-line">
            <Row label="Public URL">
              {network ? <CopyField label="Public URL" value={network.public_server_url} /> : <Skeleton className="h-8" />}
            </Row>
            <Row label="Software version">
              <span className="font-mono text-xs">{status?.server_version ?? "—"}</span>
            </Row>
            <Row label="Setup">
              {status ? (
                <Badge tone={status.setup_complete ? "ok" : "warn"}>{status.setup_complete ? "Complete" : "Incomplete"}</Badge>
              ) : (
                <Skeleton className="h-4 w-20" />
              )}
            </Row>
          </dl>
        </Panel>

        <Panel>
          <PanelHeader title="Security" action={<Badge tone="neutral" dotted={false}>Read-only</Badge>} />
          <dl className="divide-y divide-line">
            <Row label="Enrollment mode" note="Change enrollment policy on the coordinator host.">
              {status ? (
                status.legacy_enrollment ? (
                  <Badge tone="warn">Legacy shared secret</Badge>
                ) : (
                  <Badge tone="ok">Invitation only</Badge>
                )
              ) : (
                <Skeleton className="h-4 w-24" />
              )}
            </Row>
            <Row label="Admin session">
              <ReadOnly>
                <span>Secure HttpOnly cookie with CSRF protection</span>
              </ReadOnly>
            </Row>
            <Row label="Session expires">
              <span className="text-fg-muted">{expiresAt ? formatDateTime(expiresAt) : "—"}</span>
            </Row>
          </dl>
        </Panel>

        <Panel>
          <PanelHeader title="Administration" action={<Badge tone="accent" dotted={false}>Editable</Badge>} />
          <dl className="divide-y divide-line">
            <Row label="Appearance" note="Stored only in this browser.">
              <Segmented<ThemePref>
                label="Appearance"
                value={pref}
                onChange={setPref}
                options={[
                  { value: "dark", label: "Dark" },
                  { value: "light", label: "Light" },
                  { value: "system", label: "System" },
                ]}
              />
            </Row>
            <Row label="Session">
              <Button size="sm" onClick={() => void logout()}>Sign out</Button>
            </Row>
          </dl>
        </Panel>
      </div>
    </div>
  );
}
