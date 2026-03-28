const agentModes = [
  {
    title: "Browser Session",
    subtitle: "For humans",
    body: "Use the normal dashboard when a person is creating events and reviewing deposits manually.",
    tone: "border-slate-200 bg-slate-50",
  },
  {
    title: "PAT Bootstrap",
    subtitle: "Recommended entry point",
    body: "Create a Personal Access Token in the dashboard. Agents can use it for host APIs and to bootstrap a hardened runtime session.",
    tone: "border-indigo-200 bg-indigo-50",
  },
  {
    title: "Signed Payment Session",
    subtitle: "For payment automation",
    body: "Generate an ephemeral Ed25519 keypair, exchange the PAT for a short-lived JWT bound to that public key, then sign sensitive requests.",
    tone: "border-emerald-200 bg-emerald-50",
  },
];

const flowSteps = [
  {
    step: "01",
    title: "Enable Agent Security Mode",
    body: "Inside the app dashboard, turn on Agent Security Mode. This keeps the normal event dashboard clean for human users and exposes the professional controls only when needed.",
  },
  {
    step: "02",
    title: "Create a PAT",
    body: "Create a Personal Access Token and copy it once. This is your bootstrap credential, not your permanent high-risk payment credential.",
  },
  {
    step: "03",
    title: "Generate an ephemeral Ed25519 keypair",
    body: "Your agent runtime should generate its own short-lived keypair locally. Do not ask the host to paste a private key into the dashboard.",
  },
  {
    step: "04",
    title: "Exchange PAT for runtime JWT",
    body: "Call POST /api/agent/auth/pat with token, session_pubkey, and optional label. The JWT returned is bound to that public key.",
  },
  {
    step: "05",
    title: "Sign payment-impacting requests",
    body: "For sensitive actions like automated checkout settlement, send Authorization: Bearer <JWT> plus X-Agent-Timestamp and X-Agent-Signature.",
  },
  {
    step: "06",
    title: "Rotate and revoke aggressively",
    body: "PATs should be revocable. Runtime JWTs are short-lived. Ephemeral session keys should be disposable by design.",
  },
];

const docsLinks = [
  { label: "Agent Skill", href: "/agents/skill.md", note: "Runtime instructions for autonomous agents" },
  { label: "Heartbeat", href: "/agents/heartbeat.md", note: "Operational cadence and escalation rules" },
  { label: "OpenAPI", href: "/openapi.json", note: "Machine-readable API surface" },
  { label: "LLMs Index", href: "/llms.txt", note: "Top-level model discovery file" },
  { label: "Agent Docs Index", href: "/agent-docs/INDEX.md", note: "Full implementation references" },
];

export default function AgentsPage() {
  return (
    <main className="min-h-screen app-bg text-[var(--app-fg)]">
      <div className="mx-auto w-full max-w-6xl px-4 py-8 md:py-12 space-y-8 md:space-y-10">
        <section className="rounded-3xl border shadow-sm app-surface overflow-hidden">
          <div className="grid gap-6 p-6 md:grid-cols-[1.1fr_0.9fr] md:gap-8 md:p-10">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.28em] text-cyan-600">
                Agents · Professional Mode
              </p>
              <h1 className="mt-3 text-3xl md:text-5xl font-bold leading-tight">
                Give agents a safe, explicit operating path inside Payknot.
              </h1>
              <p className="mt-4 max-w-2xl text-sm md:text-lg app-muted">
                This page explains how to onboard an AI agent or automation
                runtime without weakening the payment surface. The goal is easy
                bootstrap, short-lived runtime identity, and signed high-risk
                actions.
              </p>
              <div className="mt-6 grid gap-2 md:flex md:flex-wrap md:gap-3 text-center">
                <a
                  href="/app"
                  className="btn-anim rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-2.5 font-semibold"
                >
                  Open Dashboard
                </a>
                <a
                  href="#flow"
                  className="btn-anim rounded-lg border px-5 py-2.5 font-semibold"
                >
                  See Setup Flow
                </a>
              </div>
            </div>

            <div className="rounded-2xl border border-slate-200 bg-gradient-to-br from-slate-50 to-cyan-50 p-5">
              <p className="text-sm font-semibold">Recommended policy</p>
              <div className="mt-4 space-y-3">
                <div className="rounded-xl border border-slate-200 bg-white p-3">
                  <p className="text-sm font-medium">Normal host work</p>
                  <p className="mt-1 text-xs app-muted">
                    Stay in the default event dashboard. No agent controls are
                    shown unless the host opts into Agent Security Mode.
                  </p>
                </div>
                <div className="rounded-xl border border-indigo-200 bg-indigo-50 p-3">
                  <p className="text-sm font-medium text-indigo-700">
                    Agent bootstrap
                  </p>
                  <p className="mt-1 text-xs text-indigo-900/80">
                    PAT is the onboarding credential because it is easy to issue
                    and easy to store safely in a secret manager.
                  </p>
                </div>
                <div className="rounded-xl border border-emerald-200 bg-emerald-50 p-3">
                  <p className="text-sm font-medium text-emerald-700">
                    Payment automation
                  </p>
                  <p className="mt-1 text-xs text-emerald-900/80">
                    JWT + Ed25519 signed request is required for the hardened
                    path.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-3">
          {agentModes.map((mode) => (
            <article
              key={mode.title}
              className={`rounded-2xl border p-5 shadow-sm ${mode.tone}`}
            >
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                {mode.subtitle}
              </p>
              <h2 className="mt-2 text-lg font-semibold">{mode.title}</h2>
              <p className="mt-2 text-sm app-muted">{mode.body}</p>
            </article>
          ))}
        </section>

        <section
          id="flow"
          className="rounded-3xl border app-surface p-6 md:p-8 shadow-sm"
        >
          <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
            <h2 className="text-2xl font-semibold">Agent setup flow</h2>
            <p className="text-sm app-muted">
              The shortest correct path for a secure agent integration.
            </p>
          </div>
          <div className="mt-5 grid gap-4 md:grid-cols-2">
            {flowSteps.map((item) => (
              <article
                key={item.step}
                className="rounded-2xl border border-slate-200 bg-slate-50 p-5"
              >
                <p className="text-xs font-semibold tracking-wide text-indigo-600">
                  STEP {item.step}
                </p>
                <h3 className="mt-2 text-lg font-semibold">{item.title}</h3>
                <p className="mt-2 text-sm app-muted">{item.body}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-[1fr_0.95fr]">
          <article className="rounded-3xl border app-surface p-6 md:p-8 shadow-sm">
            <h2 className="text-2xl font-semibold">Canonical signed request</h2>
            <p className="mt-3 text-sm app-muted">
              For high-risk agent actions, Payknot expects the request to be
              signed using the JWT-bound Ed25519 private key.
            </p>
            <pre className="mt-4 overflow-auto rounded-2xl border bg-slate-950 px-4 py-4 text-xs leading-6 text-cyan-100">
{`POST
/api/agent/checkout/create
<UNIX_TIMESTAMP>
<SHA256_HEX_OF_RAW_BODY>`}
            </pre>
            <p className="mt-4 text-sm app-muted">
              Headers:
            </p>
            <pre className="mt-2 overflow-auto rounded-2xl border bg-slate-950 px-4 py-4 text-xs leading-6 text-cyan-100">
{`Authorization: Bearer <agent_jwt>
X-Agent-Timestamp: <unix_seconds>
X-Agent-Signature: <base64_ed25519_signature>`}
            </pre>
          </article>

          <article className="rounded-3xl border app-surface p-6 md:p-8 shadow-sm">
            <h2 className="text-2xl font-semibold">Documentation links</h2>
            <div className="mt-5 space-y-3">
              {docsLinks.map((item) => (
                <a
                  key={item.href}
                  href={item.href}
                  className="block rounded-2xl border border-slate-200 bg-slate-50 p-4 hover:border-indigo-300 transition"
                >
                  <p className="text-sm font-semibold">{item.label}</p>
                  <p className="mt-1 text-xs app-muted">{item.note}</p>
                  <p className="mt-2 text-xs font-mono text-slate-500">
                    {item.href}
                  </p>
                </a>
              ))}
            </div>
          </article>
        </section>

        <section className="rounded-3xl border border-indigo-300/70 bg-gradient-to-r from-indigo-50 to-cyan-50 p-6 md:p-8 shadow-sm">
          <h2 className="text-2xl font-semibold">
            Use agents deliberately, not implicitly.
          </h2>
          <p className="mt-2 max-w-3xl text-sm md:text-base app-muted">
            Payknot now keeps the default dashboard clean for normal hosts and
            moves professional agent controls behind an explicit Agent Security
            Mode toggle. That separation is intentional because payment
            automation deserves a sharper security boundary.
          </p>
          <div className="mt-5 grid gap-2 md:flex md:flex-wrap md:gap-3 text-center">
            <a
              href="/app"
              className="btn-anim rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-2.5 font-semibold"
            >
              Open Agent Security Mode
            </a>
            <a
              href="/agents/skill.md"
              className="btn-anim rounded-lg border px-5 py-2.5 font-semibold"
            >
              Read Skill.md
            </a>
          </div>
        </section>
      </div>
    </main>
  );
}
