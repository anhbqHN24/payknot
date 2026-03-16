export default function LandingPage() {
  return (
    <main className="min-h-screen app-bg text-[var(--app-fg)]">
      <section className="mx-auto max-w-6xl px-4 py-16 md:py-24">
        <div className="rounded-2xl app-surface border p-8 md:p-12 shadow-sm">
          <p className="text-xs font-semibold tracking-wider text-indigo-500 uppercase">
            Payknot
          </p>
          <h1 className="mt-3 text-3xl md:text-5xl font-bold leading-tight">
            Event deposits, without payment chaos.
          </h1>
          <p className="mt-4 text-sm md:text-lg app-muted max-w-3xl">
            Payknot helps event creators collect USDC deposits with clear participant status,
            on-chain verification, and review workflows that prevent disputes.
          </p>

          <div className="mt-7 flex flex-wrap gap-3">
            <a
              href="/app"
              className="rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-2.5 font-semibold"
            >
              Create Event
            </a>
            <a
              href="/checkout/non-existent-slug"
              className="rounded-lg border px-5 py-2.5 font-semibold"
            >
              View Checkout Experience
            </a>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-4 pb-16 md:pb-24 grid gap-4 md:grid-cols-3">
        <div className="rounded-xl app-surface border p-5">
          <h3 className="font-semibold">The problem</h3>
          <p className="mt-2 text-sm app-muted">
            Manual payment collection causes missing receipts, unclear participant states,
            and delayed decisions.
          </p>
        </div>
        <div className="rounded-xl app-surface border p-5">
          <h3 className="font-semibold">How Payknot works</h3>
          <p className="mt-2 text-sm app-muted">
            Create an event, share checkout link, receive deposits, verify on-chain, then
            approve or reject with a clear audit trail.
          </p>
        </div>
        <div className="rounded-xl app-surface border p-5">
          <h3 className="font-semibold">Why it matters</h3>
          <p className="mt-2 text-sm app-muted">
            Faster operations for hosts and higher trust for participants before event day.
          </p>
        </div>
      </section>
    </main>
  );
}
