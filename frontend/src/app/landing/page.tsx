export default function LandingPage() {
  const painPoints = [
    {
      title: "Cross-border settlement is messy",
      description:
        "International attendees struggle with payment rails, high fees, and currency mismatch before event day.",
    },
    {
      title: "Payment state is unclear",
      description:
        "Hosts lose time reconciling DMs, screenshots, and spreadsheets with no single source of truth.",
    },
    {
      title: "Trust drops when receipts are weak",
      description:
        "Without verifiable on-chain references, disputes around deposit status slow operations and hurt confidence.",
    },
  ];

  const steps = [
    {
      title: "Create event checkout",
      body: "Host sets USDC amount, participant input fields, and checkout expiry to create a unique, generated event checkout link.",
    },
    {
      title: "Share link with attendees",
      body: "Attendees submit details and pay via Web3 wallet of their choice or QR code scanning method, all tied to a unique on-chain reference per attendee.",
    },
    {
      title: "Track payment state",
      body: "Every deposit is visible in a clean status pipeline from pending to paid to final decision.",
    },
    {
      title: "Have a clear & auditable view of attendees deposits",
      body: "Hosts review each deposit and finalize participant outcomes with auditable history.",
    },
  ];

  return (
    <main className="min-h-screen app-bg text-[var(--app-fg)]">
      <div className="mx-auto max-w-6xl px-4 py-12 md:py-16 space-y-8 md:space-y-10">
        <section className="rounded-2xl app-surface border p-6 md:p-10 shadow-sm">
          <p className="text-xs font-semibold tracking-wide text-indigo-500 uppercase">
            Payknot · USDC payment on Solana network
          </p>
          <h1 className="mt-3 text-3xl md:text-5xl font-bold leading-tight max-w-4xl">
            Crypto-native event payment infrastructure for organizers who need
            reliable USDC deposits.
          </h1>
          <p className="mt-4 text-sm md:text-lg app-muted max-w-3xl">
            Payknot is built for Web3 event teams handling cross-border
            attendees. It gives you a clear payment pipeline with verifiable
            on-chain references and explicit participant outcomes.
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <a
              href="/app"
              className="rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-2.5 font-semibold"
            >
              Get Started
            </a>
            <a
              href="#how-it-works"
              className="rounded-lg border px-5 py-2.5 font-semibold"
            >
              See How It Works
            </a>
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-3">
          {painPoints.map((item) => (
            <article
              key={item.title}
              className="rounded-xl app-surface border p-5 shadow-sm"
            >
              <h2 className="text-base font-semibold">{item.title}</h2>
              <p className="mt-2 text-sm app-muted">{item.description}</p>
            </article>
          ))}
        </section>

        <section
          id="how-it-works"
          className="rounded-2xl app-surface border p-6 md:p-8 shadow-sm"
        >
          <h2 className="text-xl md:text-2xl font-semibold">How it works</h2>
          <div className="mt-4 grid gap-4 md:grid-cols-2">
            {steps.map((step, idx) => (
              <article key={step.title} className="rounded-xl border p-4">
                <p className="text-xs font-semibold text-indigo-500">
                  Step {idx + 1}
                </p>
                <h3 className="mt-1 font-semibold">{step.title}</h3>
                <p className="mt-2 text-sm app-muted">{step.body}</p>
              </article>
            ))}
          </div>

          <div className="mt-6 rounded-xl border p-4">
            <p className="text-sm font-semibold">Status pipeline example</p>
            <div className="mt-3 flex flex-wrap gap-2">
              <span className="status-badge pending_payment">
                pending_payment
              </span>
              <span className="status-badge paid">paid</span>
              <span className="status-badge rejected">error</span>
            </div>
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-2">
          <article className="rounded-xl app-surface border p-5 shadow-sm">
            <h2 className="text-base font-semibold">Where Payknot fits</h2>
            <ul className="mt-3 space-y-2 text-sm app-muted list-disc pl-4">
              <li>
                Web3/Solana-native events collecting attendee deposits in USDC.
              </li>
              <li>
                Cross-border communities that need predictable, low-friction
                settlement.
              </li>
              <li>
                Teams that need operational clarity for participant payment
                states.
              </li>
            </ul>
          </article>
          <article className="rounded-xl app-surface border p-5 shadow-sm">
            <h2 className="text-base font-semibold">Where it does not fit</h2>
            <ul className="mt-3 space-y-2 text-sm app-muted list-disc pl-4">
              <li>
                General fiat checkout for cards, bank transfers, or legacy PSP
                tooling.
              </li>
              <li>
                Multi-chain abstractions where Solana USDC is not your primary
                rail.
              </li>
              <li>
                Marketplace-style split settlements across many payout
                destinations.
              </li>
            </ul>
          </article>
        </section>

        <section className="rounded-2xl app-surface border p-6 md:p-8 shadow-sm">
          <h2 className="text-xl md:text-2xl font-semibold">
            Credibility, not hype
          </h2>
          <p className="mt-3 text-sm md:text-base app-muted max-w-3xl">
            Payknot is tested in real event workflows with reliability-first
            deployment, explicit post-deploy checks, and payment verification
            tied to on-chain references. The product is designed to reduce ops
            ambiguity, not add marketing noise.
          </p>
        </section>

        <section className="rounded-2xl app-surface border p-6 md:p-8 shadow-sm">
          <h2 className="text-xl md:text-2xl font-semibold">FAQ</h2>
          <div className="mt-4 space-y-4">
            <div>
              <h3 className="font-semibold">Is Payknot fiat checkout?</h3>
              <p className="mt-1 text-sm app-muted">
                No. Payknot is focused on USDC on Solana for crypto-native event
                deposits.
              </p>
            </div>
            <div>
              <h3 className="font-semibold">
                Can attendees verify payment status later?
              </h3>
              <p className="mt-1 text-sm app-muted">
                Yes. Checkout includes transaction status lookup so users can
                confirm completed payments.
              </p>
            </div>
            <div>
              <h3 className="font-semibold">
                Do organizers get payment observability?
              </h3>
              <p className="mt-1 text-sm app-muted">
                Yes. Organizers can monitor transaction outcomes with clear
                success and error states for operational visibility.
              </p>
            </div>
          </div>
        </section>

        <section className="rounded-2xl border border-indigo-300/70 dark:border-indigo-700/50 bg-gradient-to-r from-indigo-50 to-cyan-50 dark:from-indigo-950/35 dark:to-cyan-950/30 p-6 md:p-8 shadow-sm">
          <h2 className="text-xl md:text-2xl font-semibold">
            Ready to stop payment confusion?
          </h2>
          <p className="mt-2 text-sm md:text-base app-muted max-w-2xl">
            Start with a single event and run your next deposit collection with
            verifiable, status-driven operations.
          </p>
          <div className="mt-5 flex flex-wrap gap-3">
            <a
              href="/app"
              className="rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-2.5 font-semibold"
            >
              Get Started
            </a>
            <a
              href="#how-it-works"
              className="rounded-lg border px-5 py-2.5 font-semibold"
            >
              See How It Works
            </a>
          </div>
        </section>
      </div>
    </main>
  );
}
