const statusItems = [
  {
    label: "Pending",
    detail: "Awaiting on-chain confirmation",
    cls: "pending_payment",
  },
  { label: "Paid", detail: "Confirmed payment reference", cls: "paid" },
  { label: "Error", detail: "Failed or cancelled payment", cls: "rejected" },
];

const painPoints = [
  {
    title: "Cross-border payments break momentum",
    body: "Attendees across countries hit inconsistent rails, delays, and mismatched payment expectations right before an event.",
  },
  {
    title: "Operations become spreadsheet-driven",
    body: "Teams reconcile screenshots, wallet messages, and manual logs instead of seeing one real-time payment surface.",
  },
  {
    title: "Trust drops when payment proof is unclear",
    body: "Without reference-linked transaction visibility, organizers and attendees lose confidence during payment disputes.",
  },
];

const howItWorks = [
  {
    step: "01",
    title: "Create a Solana-native checkout",
    body: "Define amount, required participant fields, and checkout expiry. Share one clean URL per event.",
    videoUrl:
      "https://videos.pexels.com/video-files/3129957/3129957-uhd_2560_1440_25fps.mp4",
    posterUrl:
      "https://images.unsplash.com/photo-1555066931-4365d14bab8c?auto=format&fit=crop&w=1000&q=80",
  },
  {
    step: "02",
    title: "Collect deposits via wallet or QR",
    body: "Attendees pay USDC on Solana with reference-linked transactions for clear attribution.",
    videoUrl:
      "https://videos.pexels.com/video-files/6963744/6963744-uhd_2560_1440_25fps.mp4",
    posterUrl:
      "https://images.unsplash.com/photo-1551288049-bebda4e38f71?auto=format&fit=crop&w=1000&q=80",
  },
  {
    step: "03",
    title: "Monitor payment lifecycle clearly",
    body: "Track each transaction as Pending, Paid, or Error with lookup support for attendees.",
    videoUrl:
      "https://videos.pexels.com/video-files/5849934/5849934-uhd_2560_1440_25fps.mp4",
    posterUrl:
      "https://images.unsplash.com/photo-1556742049-0cfed4f6a45d?auto=format&fit=crop&w=1000&q=80",
  },
  {
    step: "04",
    title: "Operate with reliability-first signals",
    body: "Use transaction observability for event readiness checks without guessing payment state.",
    videoUrl:
      "https://videos.pexels.com/video-files/7567474/7567474-uhd_2560_1440_25fps.mp4",
    posterUrl:
      "https://images.unsplash.com/photo-1681825984459-47ee999da245?auto=format&fit=crop&w=1000&q=80",
  },
];

const faqItems = [
  {
    q: "Is Payknot a generic fiat checkout tool?",
    a: "No. Payknot is built for USDC on Solana event deposit flows.",
  },
  {
    q: "Can attendees verify payments after checkout?",
    a: "Yes. Checkout keeps a transaction lookup path so attendees can verify completed payment state.",
  },
  {
    q: "What does organizer visibility look like?",
    a: "Organizers get practical observability on payment outcomes: Pending, Paid, and Error.",
  },
];

import Image from "next/image";
import Header from "@/components/Header";

export default function LandingPage() {
  return (
    <main className="min-h-screen app-bg text-[var(--app-fg)]">
      <div className="mx-auto w-full max-w-6xl px-4 py-8 md:py-12 space-y-8 md:space-y-10">
        <Header />

        <section className="relative overflow-hidden rounded-3xl border shadow-sm app-surface">
          <div className="absolute inset-0">
            <img
              src="https://images.unsplash.com/photo-1668788142072-44bd6c902e5f?auto=format&fit=crop&w=1800&q=80"
              alt="Payment infrastructure visual"
              className="h-full w-full object-cover"
            />
            <div className="absolute inset-0 bg-slate-950/55" />
          </div>

          <div className="relative grid gap-6 p-6 md:grid-cols-[1.2fr_0.8fr] md:gap-8 md:p-10">
            <div>
              <p className="text-xs font-semibold uppercase tracking-widest text-cyan-300">
                Payknot · USDC on Solana
              </p>
              <h1 className="mt-3 text-3xl font-bold leading-tight text-white md:text-5xl">
                Premium payment clarity for crypto-native events.
              </h1>
              <p className="mt-4 max-w-2xl text-sm md:text-lg text-slate-100/90">
                Payknot helps Web3 organizers run deposit collection with
                auditable, reference-linked transaction visibility. Built for
                trust-first operations, not hype-first marketing.
              </p>
              <div className="mt-6 grid gap-2 md:flex md:flex-wrap md:gap-3 text-center">
                <a
                  href="/app"
                  className="btn-anim rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-2.5 font-semibold"
                >
                  Get Started
                </a>
                <a
                  href="#how-it-works"
                  className="btn-anim rounded-lg border border-white/40 bg-white/10 text-white hover:bg-white/20 px-5 py-2.5 font-semibold"
                >
                  See How It Works
                </a>
              </div>
            </div>

            <div className="rounded-2xl border border-white/20 bg-slate-950/65 p-4 md:p-5 text-slate-100">
              <div className="flex items-center gap-3">
                <img
                  src="/payknot_icon.svg"
                  alt="Payknot"
                  className="h-9 w-9"
                />
                <div>
                  <p className="text-sm font-semibold">Payment pipeline</p>
                  <p className="text-xs text-slate-300">
                    Live operational view
                  </p>
                </div>
              </div>
              <div className="mt-4 space-y-2">
                {statusItems.map((item) => (
                  <div
                    key={item.label}
                    className="flex items-center justify-between rounded-lg border border-white/15 bg-white/5 px-3 py-2"
                  >
                    <div>
                      <p className="text-sm font-medium">{item.label}</p>
                      <p className="text-[11px] text-slate-300">
                        {item.detail}
                      </p>
                    </div>
                    <span className={`status-badge ${item.cls}`}>
                      {item.label}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section
          id="problem"
          className="grid gap-4 md:grid-cols-3 scroll-mt-[20rem] md:scroll-mt-24"
        >
          {painPoints.map((item) => (
            <article
              key={item.title}
              className="rounded-2xl border app-surface p-5 shadow-sm"
            >
              <h2 className="text-base font-semibold">{item.title}</h2>
              <p className="mt-2 text-sm app-muted">{item.body}</p>
            </article>
          ))}
        </section>

        <section
          id="how-it-works"
          className="rounded-3xl border app-surface p-6 md:p-8 shadow-sm scroll-mt-[20rem] md:scroll-mt-24"
        >
          <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
            <h2 className="text-2xl font-semibold">How it works</h2>
            <p className="text-sm app-muted">
              Built for clear payment operations, step by step.
            </p>
          </div>

          <div className="mt-5 grid gap-4 md:grid-cols-2">
            {howItWorks.map((item) => (
              <article
                key={item.step}
                className="overflow-hidden rounded-2xl border app-surface"
              >
                <div className="aspect-video bg-slate-200/40 dark:bg-slate-900/40">
                  <video
                    className="h-full w-full object-cover"
                    src={item.videoUrl}
                    poster={item.posterUrl}
                    autoPlay
                    muted
                    loop
                    playsInline
                  />
                </div>
                <div className="p-4">
                  <p className="text-xs font-semibold tracking-wide text-indigo-500">
                    STEP {item.step}
                  </p>
                  <h3 className="mt-1 font-semibold">{item.title}</h3>
                  <p className="mt-2 text-sm app-muted">{item.body}</p>
                </div>
              </article>
            ))}
          </div>
        </section>

        <section
          id="fit"
          className="grid gap-4 md:grid-cols-2 scroll-mt-[20rem] md:scroll-mt-24"
        >
          <article className="rounded-2xl border app-surface p-5 shadow-sm">
            <h2 className="text-base font-semibold">Where Payknot fits</h2>
            <ul className="mt-3 list-disc pl-4 space-y-2 text-sm app-muted">
              <li>
                Web3/Solana event teams collecting attendee deposits in USDC.
              </li>
              <li>
                Cross-border communities that need predictable payment
                visibility.
              </li>
              <li>
                Operational teams that care about auditable transaction state.
              </li>
            </ul>
          </article>
          <article className="rounded-2xl border app-surface p-5 shadow-sm">
            <h2 className="text-base font-semibold">Where it does not fit</h2>
            <ul className="mt-3 list-disc pl-4 space-y-2 text-sm app-muted">
              <li>Card and fiat-heavy checkout requirements.</li>
              <li>Multi-chain abstraction-first payment routing.</li>
              <li>Complex split-settlement marketplace payouts.</li>
            </ul>
          </article>
        </section>

        <section
          id="credibility"
          className="rounded-3xl border app-surface p-6 md:p-8 shadow-sm scroll-mt-[20rem] md:scroll-mt-24"
        >
          <h2 className="text-2xl font-semibold">Credibility, not noise</h2>
          <p className="mt-3 max-w-3xl text-sm md:text-base app-muted">
            Payknot is tested in real deployment conditions with
            reliability-first workflows, explicit sanity checks, and practical
            observability around payment outcomes. The goal is calmer event
            operations, not vanity metrics.
          </p>
        </section>

        <section
          id="faq"
          className="rounded-3xl border app-surface p-6 md:p-8 shadow-sm scroll-mt-[20rem] md:scroll-mt-24"
        >
          <h2 className="text-2xl font-semibold">FAQ</h2>
          <div className="mt-5 space-y-4">
            {faqItems.map((item) => (
              <div key={item.q}>
                <h3 className="font-semibold">{item.q}</h3>
                <p className="mt-1 text-sm app-muted">{item.a}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="rounded-3xl border border-indigo-300/70 dark:border-indigo-700/50 bg-gradient-to-r from-indigo-50 to-cyan-50 dark:from-indigo-950/35 dark:to-cyan-950/30 p-6 md:p-8 shadow-sm">
          <h2 className="text-2xl font-semibold">
            Run your next event with payment confidence.
          </h2>
          <p className="mt-2 max-w-2xl text-sm md:text-base app-muted">
            Start with one event, one checkout link, and one trustworthy payment
            lifecycle.
          </p>
          <div className="mt-5 grid gap-2 md:flex md:flex-wrap md:gap-3 text-center">
            <a
              href="/app"
              className="btn-anim rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-2.5 font-semibold"
            >
              Get Started
            </a>
            <a
              href="#how-it-works"
              className="btn-anim rounded-lg border px-5 py-2.5 font-semibold"
            >
              See How It Works
            </a>
          </div>
        </section>
      </div>
    </main>
  );
}
