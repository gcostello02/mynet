import {
  AlertTriangle,
  CheckCircle2,
  MonitorSmartphone,
  Server,
  ShieldCheck,
  Ticket,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { ApiError, api } from "../api/client";
import { Logo } from "../components/brand/Logo";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { CopyButton, CopyField } from "../components/ui/Copy";
import { Input } from "../components/ui/Input";
import { Panel, PanelHeader } from "../components/ui/Card";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";
import { joinCommand } from "../lib/invitation";
import { shortFingerprint } from "../lib/time";

type Step = "welcome" | "bootstrap" | "network" | "identity" | "invite" | "join";

const steps: { id: Step; label: string }[] = [
  { id: "welcome", label: "Welcome" },
  { id: "bootstrap", label: "Verify host" },
  { id: "network", label: "Network" },
  { id: "identity", label: "Identity" },
  { id: "invite", label: "Invitation" },
  { id: "join", label: "Join device" },
];

export function SetupWizardPage({ onInitialized }: { onInitialized: () => void }) {
  const navigate = useNavigate();
  const { login } = useAuth();
  const { push } = useToast();
  const [step, setStep] = useState<Step>("welcome");
  const [status, setStatus] = useState<Awaited<ReturnType<typeof api.setupStatus>> | null>(null);
  const [bootstrap, setBootstrap] = useState("");
  const [networkName, setNetworkName] = useState("My network");
  const [publicUrl, setPublicUrl] = useState(() => `${window.location.origin}`);
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fingerprint, setFingerprint] = useState("");
  const [inviteCompact, setInviteCompact] = useState<string | null>(null);
  const [inviteLabel, setInviteLabel] = useState("First device");
  const [enrolled, setEnrolled] = useState(false);

  const refreshStatus = useCallback(() => {
    api.setupStatus().then(setStatus).catch(() => setStatus(null));
  }, []);
  useEffect(() => {
    refreshStatus();
  }, [refreshStatus]);

  const stepIndex = steps.findIndex((s) => s.id === step);

  const bootstrapHint = useMemo(() => {
    if (!status?.bootstrap_expires_at) return null;
    return `Capability expires ${new Date(status.bootstrap_expires_at).toLocaleString()}.`;
  }, [status]);

  async function submitNetwork(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const res = await api.setup(
        {
          username: username.trim(),
          password,
          network_display_name: networkName.trim(),
          public_server_url: publicUrl.trim(),
        },
        bootstrap.trim(),
      );
      setFingerprint(res.coordinator_fingerprint);
      await login(username.trim(), password);
      setPassword("");
      setStep("identity");
      onInitialized();
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 403) setError("Bootstrap capability rejected or setup already completed.");
        else if (err.status === 410 || err.message.includes("expired")) setError("Bootstrap capability expired. Restart the coordinator and copy a new capability from its console.");
        else setError(err.message);
      } else {
        setError("Could not reach the coordinator.");
      }
    } finally {
      setBusy(false);
    }
  }

  async function createInvite() {
    setBusy(true);
    setError(null);
    try {
      const res = await api.createInvitation({
        label: inviteLabel.trim() || "First device",
        expires_in_seconds: 86400,
        max_uses: 1,
      });
      const invitation = res.invitation?.trim();
      if (!invitation) {
        setError("Coordinator did not return a join invitation. Rebuild the coordinator and try again.");
        return;
      }
      setInviteCompact(invitation);
      setStep("join");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not create invitation.");
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    if (step !== "join" || !enrolled) {
      const t = window.setInterval(() => {
        api
          .devices()
          .then((d) => {
            if (d.length > 0) setEnrolled(true);
          })
          .catch(() => {});
      }, 5000);
      return () => window.clearInterval(t);
    }
  }, [step, enrolled]);

  const joinCmd = inviteCompact ? joinCommand(inviteCompact, "my-device") : "";

  return (
    <div className="min-h-screen bg-canvas">
      <header className="border-b border-line bg-panel px-5 py-4">
        <div className="mx-auto flex max-w-3xl items-center justify-between gap-4">
          <Logo />
          <span className="text-xs text-fg-muted">First-run setup</span>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-5 py-8">
        <ol className="mb-8 flex flex-wrap gap-2" aria-label="Setup progress">
          {steps.map((s, i) => (
            <li
              key={s.id}
              className={`rounded-full px-2.5 py-1 text-[11px] font-medium ${
                i === stepIndex ? "bg-accent-soft text-accent" : i < stepIndex ? "text-fg-muted" : "text-fg-subtle"
              }`}
            >
              {s.label}
            </li>
          ))}
        </ol>

        {error && (
          <p role="alert" className="mb-4 rounded-md border border-bad/30 bg-bad-soft px-3 py-2 text-[13px] text-bad">
            {error}
          </p>
        )}

        {step === "welcome" && (
          <Panel className="p-6 sm:p-8">
            <h1 className="text-xl font-semibold tracking-tight">Set up your coordinator</h1>
            <p className="mt-2 max-w-xl text-[13px] leading-relaxed text-fg-muted">
              MyNet is fully self-hosted. This wizard runs on your coordinator and creates your network identity,
              administrator account, and first device invitation. No MyNet cloud account is involved.
            </p>
            <ul className="mt-6 space-y-3 text-[13px] text-fg-muted">
              <li className="flex gap-2">
                <Server className="mt-0.5 h-4 w-4 shrink-0 text-accent" aria-hidden />
                You control the database, coordinator keys, and every enrolled device.
              </li>
              <li className="flex gap-2">
                <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-accent" aria-hidden />
                Remote setup requires a one-time bootstrap capability from the coordinator host console.
              </li>
            </ul>
            <Button variant="primary" className="mt-8" onClick={() => setStep("bootstrap")}>
              Begin setup
            </Button>
          </Panel>
        )}

        {step === "bootstrap" && (
          <Panel className="p-6 sm:p-8">
            <h2 className="text-lg font-semibold">Prove coordinator access</h2>
            <p className="mt-2 text-[13px] leading-relaxed text-fg-muted">
              On the machine running <code className="rounded bg-raised px-1 font-mono text-xs">mynet-server</code>,
              copy the bootstrap capability printed at startup (or run{" "}
              <code className="rounded bg-raised px-1 font-mono text-xs">mynet admin bootstrap --server http://127.0.0.1:8080</code>{" "}
              over SSH). Paste it here. It is used once and never stored in your browser.
            </p>
            {status && !status.bootstrap_active && (
              <p className="mt-4 rounded-md border border-warn/30 bg-warn-soft px-3 py-2 text-[13px] text-warn">
                No active bootstrap capability. Restart the coordinator to issue a new one.
              </p>
            )}
            <form
              className="mt-6 space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                if (bootstrap.trim().length < 16) {
                  setError("Paste the full bootstrap capability from the coordinator console.");
                  return;
                }
                setError(null);
                setStep("network");
              }}
            >
              <Input
                label="Bootstrap capability"
                hint={bootstrapHint ?? "Shown once in the coordinator console when the database is uninitialized."}
                value={bootstrap}
                onChange={(e) => setBootstrap(e.target.value)}
                className="font-mono text-xs"
                autoComplete="off"
                spellCheck={false}
                required
              />
              <div className="flex flex-wrap gap-2">
                <Button type="button" onClick={() => setStep("welcome")}>Back</Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    setError(null);
                    fetch("/v1/setup/bootstrap", { credentials: "include" })
                      .then(async (res) => {
                        if (!res.ok) throw new Error("Only available on the coordinator host.");
                        const j = (await res.json()) as { bootstrap_token: string };
                        setBootstrap(j.bootstrap_token);
                        push("Loaded bootstrap capability from coordinator.", "success");
                      })
                      .catch(() => setError("Could not load capability. Copy it from the coordinator console instead."));
                  }}
                >
                  Load from coordinator host
                </Button>
                <Button variant="primary" type="submit">Continue</Button>
              </div>
            </form>
          </Panel>
        )}

        {step === "network" && (
          <Panel className="p-6 sm:p-8">
            <h2 className="text-lg font-semibold">Network and administrator</h2>
            <form onSubmit={submitNetwork} className="mt-6 space-y-4">
              <Input label="Network name" value={networkName} onChange={(e) => setNetworkName(e.target.value)} required maxLength={80} />
              <Input
                label="Public coordinator URL"
                hint="HTTPS URL devices use to reach this coordinator (via your reverse proxy)."
                value={publicUrl}
                onChange={(e) => setPublicUrl(e.target.value)}
                required
                className="font-mono text-xs"
              />
              <Input label="Administrator username" value={username} onChange={(e) => setUsername(e.target.value)} required minLength={3} autoComplete="username" />
              <Input
                label="Administrator password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={12}
                autoComplete="new-password"
                hint="At least 12 characters."
              />
              <div className="flex gap-2 pt-2">
                <Button type="button" onClick={() => setStep("bootstrap")}>Back</Button>
                <Button variant="primary" type="submit" loading={busy}>Create network</Button>
              </div>
            </form>
          </Panel>
        )}

        {step === "identity" && (
          <Panel className="p-6 sm:p-8">
            <h2 className="text-lg font-semibold">Coordinator identity</h2>
            <p className="mt-2 text-[13px] text-fg-muted">
              Devices verify this fingerprint when they join. Back up the coordinator database and{" "}
              <code className="rounded bg-raised px-1 font-mono text-xs">coordinator.ed25519</code> beside it.
            </p>
            <div className="mt-4">
              <CopyField label="Fingerprint" value={fingerprint} />
            </div>
            <div className="mt-4 flex gap-2.5 rounded-md border border-warn/30 bg-warn-soft px-3 py-2.5 text-[13px] text-warn">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
              <p>
                Losing the database or private key means devices can no longer trust this coordinator. Store offline
                backups before enrolling production devices.
              </p>
            </div>
            <Button variant="primary" className="mt-6" onClick={() => setStep("invite")}>
              Continue
            </Button>
          </Panel>
        )}

        {step === "invite" && (
          <Panel className="p-6 sm:p-8">
            <h2 className="text-lg font-semibold">First invitation</h2>
            <p className="mt-2 text-[13px] text-fg-muted">
              Create a one-time enrollment secret for your first device. You will see it only once.
            </p>
            <Input className="mt-4" label="Label (optional)" value={inviteLabel} onChange={(e) => setInviteLabel(e.target.value)} />
            <Button variant="primary" className="mt-6" loading={busy} onClick={() => void createInvite()}>
              <Ticket className="h-3.5 w-3.5" aria-hidden /> Create invitation
            </Button>
          </Panel>
        )}

        {step === "join" && inviteCompact && (
          <div className="space-y-4">
            <Panel className="p-6">
              <PanelHeader
                title="Join invitation"
                description="Copy the full invitation or join command before continuing. It is not saved in this browser."
              />
              <code className="block break-all rounded-md border border-line-strong bg-canvas p-3 font-mono text-xs leading-5">
                {inviteCompact}
              </code>
              <div className="mt-3 flex flex-wrap gap-2">
                <CopyButton value={inviteCompact} label="Copy invitation" variant="primary" />
                <CopyButton value={joinCmd} label="Copy join command" />
              </div>
            </Panel>

            <Panel className="p-6">
              <h3 className="text-[13px] font-medium">On macOS or Linux</h3>
              <ol className="mt-3 list-decimal space-y-2 pl-5 text-[13px] leading-relaxed text-fg-muted">
                <li>Install the MyNet client and ensure the coordinator URL uses HTTPS.</li>
                <li>
                  When prompted, confirm the coordinator fingerprint matches{" "}
                  <span className="font-mono text-fg">{shortFingerprint(fingerprint)}</span>.
                </li>
                <li>
                  Run the join command in a terminal (installs a privileged daemon when you approve):
                  <pre className="mt-2 overflow-x-auto rounded-md border border-line bg-canvas p-3 font-mono text-xs text-fg">
                    {joinCmd}
                  </pre>
                </li>
                <li>Start the client service: <code className="font-mono text-xs">mynet service start</code></li>
              </ol>
            </Panel>

            <Panel className="p-6">
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <MonitorSmartphone className="h-4 w-4 text-fg-muted" aria-hidden />
                  <span className="text-[13px] font-medium">Enrollment status</span>
                </div>
                {enrolled ? <Badge tone="ok">Device enrolled</Badge> : <Badge tone="neutral">Waiting…</Badge>}
              </div>
              {enrolled && (
                <p className="mt-3 flex items-center gap-2 text-[13px] text-ok">
                  <CheckCircle2 className="h-4 w-4" aria-hidden /> A device appears in the coordinator. Open the dashboard to manage it.
                </p>
              )}
            </Panel>

            <Button variant="primary" onClick={() => navigate("/")}>
              Open dashboard
            </Button>
          </div>
        )}
      </main>
    </div>
  );
}
