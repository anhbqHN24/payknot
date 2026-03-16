"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { useParams } from "next/navigation";
import { useConnection, useWallet } from "@solana/wallet-adapter-react";
import { PublicKey } from "@solana/web3.js";
import { Select, Steps, message } from "antd";
import { createUsdcTransfer } from "@/lib/solana";

const WalletMultiButton = dynamic(
  () =>
    import("@solana/wallet-adapter-react-ui").then(
      (mod) => mod.WalletMultiButton,
    ),
  { ssr: false },
);

type ParticipantField = {
  field_name: string;
  required: boolean;
};

type EventData = {
  slug: string;
  title: string;
  description: string;
  eventImageUrl: string;
  eventDate: string;
  checkoutExpiresAt?: string;
  location: string;
  organizerName: string;
  merchantWallet: string;
  amountUsdc: string;
  amountRaw: number;
  network: string;
  participantForm: ParticipantField[];
  paymentMethodWallet: boolean;
  paymentMethodQr: boolean;
};

type CheckoutStatus = {
  reference: string;
  status: string;
  signature: string;
  network: string;
  solscanUrl: string;
  paymentMethod: "wallet" | "qr";
  participantData?: Record<string, string>;
};

type PaymentMethod = "wallet" | "qr";

type PendingSession = {
  sessionId?: string;
  reference: string;
  expiresAt: number;
  signature?: string;
  method: PaymentMethod;
};

function formatSeconds(seconds: number) {
  const m = Math.floor(seconds / 60)
    .toString()
    .padStart(2, "0");
  const s = Math.floor(seconds % 60)
    .toString()
    .padStart(2, "0");
  return `${m}:${s}`;
}

function mintFromEnv() {
  return (
    process.env.NEXT_PUBLIC_USDC_MINT ||
    "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
  );
}

function isHttpLocation(value: string) {
  if (!value) return false;
  return /^https?:\/\//i.test(value);
}

function methodLabel(method?: string) {
  return method === "qr" ? "QR code" : "Wallet connection";
}

function looksLikeHtml(value: string) {
  return /<[^>]+>/.test(value || "");
}

function sanitizeImportedHtml(input: string) {
  if (!input) return "";
  return input
    .replace(/<script[\s\S]*?>[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?>[\s\S]*?<\/style>/gi, "")
    .replace(/\son\w+="[^"]*"/gi, "")
    .replace(/\son\w+='[^']*'/gi, "");
}

function CheckoutInner() {
  const { slug } = useParams<{ slug: string }>();
  const { connection } = useConnection();
  const { publicKey, sendTransaction } = useWallet();
  const popupRef = useRef<Window | null>(null);
  const lookupSectionRef = useRef<HTMLElement | null>(null);
  const step3SectionRef = useRef<HTMLElement | null>(null);

  const [eventData, setEventData] = useState<EventData | null>(null);
  const [participantData, setParticipantData] = useState<
    Record<string, string>
  >({});
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>("wallet");
  const [sessionMethod, setSessionMethod] = useState<PaymentMethod | null>(
    null,
  );
  const [loadingEvent, setLoadingEvent] = useState(true);
  const [loadingPay, setLoadingPay] = useState(false);
  const [checkingLookup, setCheckingLookup] = useState(false);
  const [lookupEmail, setLookupEmail] = useState("");
  const [lookupError, setLookupError] = useState("");
  const [highlightLookup, setHighlightLookup] = useState(false);
  const [highlightStep3, setHighlightStep3] = useState(false);
  const [activeSessionId, setActiveSessionId] = useState("");
  const [reference, setReference] = useState("");
  const [statusData, setStatusData] = useState<CheckoutStatus | null>(null);
  const [showFullSignature, setShowFullSignature] = useState(false);

  const checkoutStatusClass = (status?: string) => {
    const s = (status || "pending_payment").toLowerCase();
    if (s === "paid" || s === "approved") return "status-badge paid";
    if (s === "rejected" || s === "failed" || s === "cancelled")
      return "status-badge rejected";
    return "status-badge pending_payment";
  };
  const [participantErrors, setParticipantErrors] = useState<
    Record<string, string>
  >({});
  const [timeLeft, setTimeLeft] = useState<number | null>(null);
  const [step, setStep] = useState(0);
  const [error, setError] = useState("");

  const storageKey = `checkout_pending_${slug}`;

  useEffect(() => {
    (async () => {
      try {
        setLoadingEvent(true);
        const res = await fetch(`/api/checkout/${slug}`);
        if (!res.ok) throw new Error(await res.text());
        const data = (await res.json()) as EventData;
        setEventData(data);
        if (!data.paymentMethodWallet && data.paymentMethodQr) {
          setPaymentMethod("qr");
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load event");
      } finally {
        setLoadingEvent(false);
      }
    })();
  }, [slug]);

  const hydrateStatus = (data: CheckoutStatus) => {
    setStatusData(data);
    setReference(data.reference || "");
    if (data.paymentMethod) {
      setSessionMethod(data.paymentMethod);
      setPaymentMethod(data.paymentMethod);
    }
    if (data.status === "paid") {
      clearPending();
      setStep(2);
    }
  };

  const refreshStatus = async (refArg?: string) => {
    const ref = refArg || reference;
    if (!ref) return;
    const res = await fetch(`/api/checkout/status?reference=${ref}`);
    if (!res.ok) return;
    const data = (await res.json()) as CheckoutStatus;
    hydrateStatus(data);
  };

  useEffect(() => {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return;
    try {
      const data = JSON.parse(raw) as PendingSession;
      if (!data.reference || !data.expiresAt || data.expiresAt <= Date.now()) {
        localStorage.removeItem(storageKey);
        return;
      }
      setActiveSessionId(data.sessionId || "");
      setReference(data.reference);
      setSessionMethod(data.method || null);
      setPaymentMethod(data.method || "wallet");
      if (data.method === "wallet") {
        setTimeLeft(Math.floor((data.expiresAt - Date.now()) / 1000));
      } else {
        setTimeLeft(null);
      }
      setStep(2);
      void refreshStatus(data.reference);
    } catch {
      localStorage.removeItem(storageKey);
    }
  }, [storageKey]);

  useEffect(() => {
    if (
      sessionMethod !== "wallet" ||
      !reference ||
      !timeLeft ||
      timeLeft <= 0
    ) {
      return;
    }
    const timer = setInterval(() => {
      setTimeLeft((prev) => (prev && prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [sessionMethod, reference, timeLeft]);

  useEffect(() => {
    const handler = (event: MessageEvent) => {
      if (!event?.data || event.data.type !== "payknot:qr-paid") return;
      const payload = event.data.payload as CheckoutStatus | undefined;
      if (!payload || payload.status !== "paid") return;
      hydrateStatus(payload);
      setError("");
    };
    window.addEventListener("message", handler);
    return () => window.removeEventListener("message", handler);
  }, []);

  const hasActiveSession = useMemo(() => {
    if (!reference) return false;
    const status = (statusData?.status || "pending_payment").toLowerCase();
    return !["paid", "failed", "cancelled", "rejected", "approved"].includes(status);
  }, [reference, statusData?.status]);

  useEffect(() => {
    const cancelPayload = reference
      ? new Blob([JSON.stringify({ reference })], {
          type: "application/json",
        })
      : null;

    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!hasActiveSession || !reference) return;
      if (cancelPayload) {
        navigator.sendBeacon("/api/checkout/cancel", cancelPayload);
      }
      localStorage.removeItem(storageKey);
      event.preventDefault();
      event.returnValue = "";
    };

    const onPageHide = () => {
      if (!hasActiveSession || !reference) return;
      if (cancelPayload) {
        navigator.sendBeacon("/api/checkout/cancel", cancelPayload);
      }
      localStorage.removeItem(storageKey);
    };

    window.addEventListener("beforeunload", onBeforeUnload);
    window.addEventListener("pagehide", onPageHide);
    return () => {
      window.removeEventListener("beforeunload", onBeforeUnload);
      window.removeEventListener("pagehide", onPageHide);
    };
  }, [hasActiveSession, reference, storageKey]);

  const displayDate = useMemo(() => {
    if (!eventData?.eventDate) return "";
    return new Date(eventData.eventDate).toLocaleString();
  }, [eventData]);

  const isCheckoutExpired = useMemo(() => {
    if (!eventData?.checkoutExpiresAt) return false;
    return new Date(eventData.checkoutExpiresAt).getTime() <= Date.now();
  }, [eventData]);

  const availableMethods = useMemo(() => {
    if (!eventData) return [] as { label: string; value: PaymentMethod }[];
    const out: { label: string; value: PaymentMethod }[] = [];
    if (eventData.paymentMethodWallet) {
      out.push({ label: "Wallet connection", value: "wallet" });
    }
    if (eventData.paymentMethodQr) {
      out.push({ label: "QR code", value: "qr" });
    }
    return out;
  }, [eventData]);

  const persistPending = (
    ref: string,
    method: PaymentMethod,
    signature?: string,
    sessionId?: string,
  ) => {
    const expiresAt = Date.now() + 10 * 60 * 1000;
    if (method === "wallet") {
      setTimeLeft(10 * 60);
    } else {
      setTimeLeft(null);
    }
    const payload: PendingSession = {
      sessionId: sessionId || "",
      reference: ref,
      expiresAt,
      signature: signature || "",
      method,
    };
    localStorage.setItem(storageKey, JSON.stringify(payload));
  };

  const clearPending = () => {
    localStorage.removeItem(storageKey);
    setActiveSessionId("");
    setReference("");
    setSessionMethod(null);
    setTimeLeft(null);
  };

  const cancelCurrentSession = async () => {
    if (!reference) return;
    try {
      if (activeSessionId) {
        await fetch(`/api/v1/payment-sessions/${activeSessionId}/cancel`, {
          method: "POST",
          keepalive: true,
        });
      }
      await fetch("/api/checkout/cancel", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reference }),
        keepalive: true,
      });
    } catch {
      // best effort on cancel
    }
    clearPending();
    setStatusData(null);
    setStep(0);
  };

  const validateStep1 = () => {
    if (!eventData) return false;
    const nextErrors: Record<string, string> = {};
    for (const field of eventData.participantForm || []) {
      const key = field.field_name;
      const value = (participantData[key] || "").trim();
      if (field.required && !value) {
        nextErrors[key] = `${key} is required`;
        continue;
      }
      if (
        value &&
        key.trim().toLowerCase() === "email" &&
        !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
      ) {
        nextErrors[key] = "Please enter a valid email address.";
      }
    }
    setParticipantErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      setError("Please fill all required fields.");
      return false;
    }
    setError("");
    return true;
  };

  const emailFromParticipantData = () => {
    const direct = (participantData["email"] || "").trim().toLowerCase();
    if (direct) return direct;
    for (const [key, value] of Object.entries(participantData)) {
      if (key.trim().toLowerCase() === "email") {
        return String(value || "")
          .trim()
          .toLowerCase();
      }
    }
    return "";
  };

  const createInvoice = async (
    walletAddress: string,
    method: PaymentMethod,
  ) => {
    const v1Res = await fetch("/api/v1/payment-sessions", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": `${slug}-${method}-${Date.now()}`,
      },
      body: JSON.stringify({
        slug,
        walletAddress,
        participantData,
        paymentMethod: method,
      }),
    });
    if (v1Res.ok) {
      const data = (await v1Res.json()) as {
        sessionId: string;
        reference: string;
        amountAtomic: number;
      };
      return {
        sessionId: data.sessionId,
        reference: data.reference,
        amountRaw: data.amountAtomic,
      };
    }

    const res = await fetch("/api/checkout/invoice", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        slug,
        walletAddress,
        participantData,
        paymentMethod: method,
      }),
    });
    if (!res.ok) throw new Error(await res.text());
    const legacy = (await res.json()) as { reference: string; amountRaw: number };
    return { sessionId: "", reference: legacy.reference, amountRaw: legacy.amountRaw };
  };

  const checkParticipantStatus = async (email: string) => {
    const res = await fetch("/api/checkout/participant-status", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slug, participantData: { email } }),
    });
    if (res.status === 204) return null;
    if (!res.ok) return null;
    return (await res.json()) as CheckoutStatus;
  };

  const startWalletPayment = async () => {
    if (!publicKey || !eventData) return;
    setLoadingPay(true);
    setError("");
    let createdReference = "";
    let createdSessionId = "";
    let sentSignature = "";
    try {
      const created = await createInvoice(publicKey.toBase58(), "wallet");
      createdReference = created.reference;
      createdSessionId = created.sessionId || "";
      setActiveSessionId(createdSessionId);
      setReference(created.reference);
      setStatusData({
        reference: created.reference,
        status: "pending_payment",
        signature: "",
        network: eventData.network,
        solscanUrl: "",
        paymentMethod: "wallet",
      });
      setSessionMethod("wallet");
      setPaymentMethod("wallet");
      persistPending(created.reference, "wallet", "", created.sessionId);

      const transaction = await createUsdcTransfer(
        connection,
        publicKey,
        new PublicKey(eventData.merchantWallet),
        Number(eventData.amountUsdc),
        created.reference,
      );
      const signature = await sendTransaction(transaction, connection, {
        skipPreflight: false,
        maxRetries: 5,
      });
      sentSignature = signature;
      persistPending(created.reference, "wallet", signature, created.sessionId);

      const confirmed = await connection.confirmTransaction(signature, "confirmed");
      if (confirmed.value.err) throw new Error("Transaction failed on-chain");

      let confirmRes: Response;
      if (created.sessionId) {
        confirmRes = await fetch(
          `/api/v1/payment-sessions/${created.sessionId}/submit-signature`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ signature }),
          },
        );
      } else {
        confirmRes = await fetch("/api/checkout/confirm", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reference: created.reference, signature }),
        });
      }
      if (!confirmRes.ok) throw new Error(await confirmRes.text());
      const paid = (await confirmRes.json()) as CheckoutStatus;
      hydrateStatus(paid);
    } catch (err) {
      if (createdReference && !sentSignature) {
        if (createdSessionId) {
          await fetch(`/api/v1/payment-sessions/${createdSessionId}/cancel`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
          });
        } else {
          await fetch("/api/checkout/cancel", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ reference: createdReference }),
          });
        }
        clearPending();
        setStatusData(null);
        setShowFullSignature(false);
      }
      setError(err instanceof Error ? err.message : "Payment failed");
    } finally {
      setLoadingPay(false);
    }
  };

  const openQrWindow = (referenceValue: string) => {
    if (!eventData) return;
    const mint = mintFromEnv();
    const expiresAt = Date.now() + 10 * 60 * 1000;
    const query = new URLSearchParams({
      amount: String(eventData.amountUsdc),
      "spl-token": mint,
      memo: referenceValue,
      label: eventData.title || "Event Deposit",
      message: `Deposit ${eventData.amountUsdc} USDC for ${eventData.title || "event"}`,
    });
    const solanaPayURL = `solana:${eventData.merchantWallet}?${query.toString()}`;
    const qrUrl = `https://api.qrserver.com/v1/create-qr-code/?size=320x320&data=${encodeURIComponent(solanaPayURL)}`;
    const checkoutURL = `${window.location.origin}/checkout/${slug}`;
    const child = window.open("", "_blank", "width=540,height=760");
    if (!child) {
      setError("Popup blocked. Please allow popups and try again.");
      return;
    }
    popupRef.current = child;

    const referenceJSON = JSON.stringify(referenceValue);
    const checkoutURLJSON = JSON.stringify(checkoutURL);
    const expiresAtJSON = JSON.stringify(expiresAt);

    child.document.write(`
      <html>
        <head>
          <title>QR Payment</title>
          <style>
            body { font-family: Arial, sans-serif; padding: 20px; line-height: 1.45; color: #0f172a; }
            .card { border: 1px solid #e2e8f0; border-radius: 12px; padding: 14px; margin-top: 12px; }
            .status { min-height: 22px; margin-top: 10px; color: #334155; }
            .ok { color: #166534; }
            .err { color: #b91c1c; }
            code { background: #f1f5f9; padding: 2px 6px; border-radius: 6px; }
          </style>
        </head>
        <body>
          <h2>Scan to Pay</h2>
          <p>Reference: <code>${referenceValue}</code></p>
          <div class="card" style="text-align:center;">
            <img src="${qrUrl}" alt="QR code" />
            <p id="timer" style="font-weight:600; margin-top: 10px;">Session time left: 10:00</p>
          </div>
          <div class="card">
            <div id="status" class="status">Waiting for payment...</div>
            <div id="result" class="status"></div>
          </div>

          <script>
            const reference = ${referenceJSON};
            const checkoutURL = ${checkoutURLJSON};
            const expiresAt = ${expiresAtJSON};
            const timerEl = document.getElementById('timer');
            const statusEl = document.getElementById('status');
            const resultEl = document.getElementById('result');
            let closed = false;
            let checking = false;
            let pollHandle = null;
            let pollDelay = 7000;

            function fmt(seconds) {
              const m = String(Math.floor(seconds / 60)).padStart(2, '0');
              const s = String(Math.max(0, Math.floor(seconds % 60))).padStart(2, '0');
              return m + ':' + s;
            }

            function finishAndClose() {
              if (closed) return;
              closed = true;
              let n = 5;
              resultEl.className = 'status ok';
              const t = setInterval(function() {
                resultEl.textContent = 'Payment confirmed. This page will close in ' + n + 's...';
                n -= 1;
                if (n < 0) {
                  clearInterval(t);
                  if (window.opener && !window.opener.closed) {
                    try {
                      window.opener.focus();
                    } catch (e) {}
                  }
                  window.close();
                }
              }, 1000);
            }

            async function detectPayment() {
              if (closed || checking) return;
              const left = Math.floor((expiresAt - Date.now()) / 1000);
              if (left <= 0) {
                statusEl.textContent = 'Session expired. Please create a new QR payment session from checkout page.';
                statusEl.className = 'status err';
                if (pollHandle) clearTimeout(pollHandle);
                return;
              }

              checking = true;
              statusEl.textContent = 'Verifying transaction on-chain...';
              statusEl.className = 'status';

              try {
                const res = await fetch('/api/checkout/detect', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ reference }),
                });
                const text = await res.text();
                let payload = null;
                try { payload = text ? JSON.parse(text) : null; } catch {}

                if (res.ok && payload && payload.status === 'paid') {
                  statusEl.textContent = 'Payment detected and verified.';
                  statusEl.className = 'status ok';
                  if (window.opener && !window.opener.closed) {
                    window.opener.postMessage({ type: 'payknot:qr-paid', payload }, '*');
                  }
                  finishAndClose();
                  return;
                }

                if (res.status === 202) {
                  statusEl.textContent = 'Waiting for transaction confirmation...';
                  statusEl.className = 'status';
                  pollDelay = 7000;
                  return;
                }

                if (res.status === 429) {
                  statusEl.textContent = 'Server is busy. Retrying in a few seconds...';
                  statusEl.className = 'status err';
                  pollDelay = 15000;
                  return;
                }

                if (res.status === 404) {
                  statusEl.textContent = text || 'Session expired. Please create a new session.';
                  statusEl.className = 'status err';
                  if (pollHandle) clearTimeout(pollHandle);
                  return;
                }

                statusEl.textContent = text || 'Verification failed. Retrying...';
                statusEl.className = 'status err';
                pollDelay = 10000;
              } catch (err) {
                statusEl.textContent = (err && err.message) ? err.message : 'Network error. Retrying...';
                statusEl.className = 'status err';
                pollDelay = 12000;
              } finally {
                checking = false;
                if (!closed) {
                  pollHandle = setTimeout(detectPayment, pollDelay);
                }
              }
            }

            function tick() {
              if (closed) return;
              const left = Math.floor((expiresAt - Date.now()) / 1000);
              if (left <= 0) {
                timerEl.textContent = 'Session expired';
                timerEl.style.color = '#b91c1c';
                return;
              }
              timerEl.textContent = 'Session time left: ' + fmt(left);
              setTimeout(tick, 1000);
            }

            tick();
            detectPayment();
          </script>
        </body>
      </html>
    `);
    child.document.close();
  };

  const startQrSession = async () => {
    if (!eventData) return;
    setLoadingPay(true);
    setError("");
    try {
      const created = await createInvoice("", "qr");
      setActiveSessionId(created.sessionId || "");
      setReference(created.reference);
      setSessionMethod("qr");
      setPaymentMethod("qr");
      setStatusData({
        reference: created.reference,
        status: "pending_payment",
        signature: "",
        network: eventData.network,
        solscanUrl: "",
        paymentMethod: "qr",
      });
      persistPending(created.reference, "qr", "", created.sessionId);
      openQrWindow(created.reference);
      setStep(2);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to create QR payment session",
      );
    } finally {
      setLoadingPay(false);
    }
  };

  if (loadingEvent) return <main className="p-6">Loading checkout...</main>;
  if (!eventData)
    return <main className="p-6 text-rose-600">Event not found</main>;

  const chosenMethod =
    statusData?.paymentMethod || sessionMethod || paymentMethod;
  const isPaid = statusData?.status === "paid";

  return (
    <main className="checkout-page mx-auto max-w-4xl p-4 md:p-6 w-full">
      <h1 className="text-2xl text-center font-semibold">{eventData.title}</h1>
      {eventData.eventImageUrl && (
        <img
          src={eventData.eventImageUrl}
          alt={eventData.title}
          className="mt-3 h-80 mx-auto rounded-xl object-cover"
        />
      )}
      {looksLikeHtml(eventData.description) ? (
        <div
          className="mt-3 text-slate-600 [&_a]:text-blue-600 [&_a]:underline [&_img]:my-3 [&_img]:max-w-full [&_img]:rounded [&_ol]:ml-5 [&_ol]:list-decimal [&_p]:my-1 [&_strong]:font-semibold [&_ul]:ml-5 [&_ul]:list-disc"
          dangerouslySetInnerHTML={{
            __html: sanitizeImportedHtml(eventData.description),
          }}
        />
      ) : (
        <p className="mt-3 whitespace-pre-wrap break-words text-slate-600">
          {eventData.description}
        </p>
      )}
      <p className="mt-1 text-sm text-slate-500">{displayDate}</p>
      {eventData.location &&
        (isHttpLocation(eventData.location) ? (
          <a
            className="mt-1 inline-block text-sm text-blue-600 underline"
            href={eventData.location}
            target="_blank"
            rel="noreferrer"
          >
            {eventData.location}
          </a>
        ) : (
          <p className="mt-1 text-sm text-slate-600">{eventData.location}</p>
        ))}
      <p className="mt-2 text-xl font-semibold text-slate-900">
        Join deposit: {eventData.amountUsdc} USDC
      </p>

      <section
        ref={lookupSectionRef}
        className={`mt-4 rounded-xl border border-slate-200 dark:border-slate-700 app-surface p-4 shadow-sm transition-all ${
          highlightLookup ? "ring-2 ring-amber-400 border-amber-400" : ""
        }`}
      >
        <h2 className="text-base font-semibold">
          Already paid? Check transaction
        </h2>
        <p className="mt-1 text-sm text-slate-600">
          This check form is separate from checkout steps. Enter email to view
          your completed transaction.
        </p>
        <div className="mt-3 flex flex-col gap-2 md:flex-row md:items-end">
          <label className="text-sm md:min-w-[320px]">
            Email
            <input
              type="email"
              value={lookupEmail}
              onChange={(e) => setLookupEmail(e.target.value)}
              placeholder="you@example.com"
              className="mt-1 w-full rounded border px-3 py-2"
            />
          </label>
          <button
            className="h-10 rounded bg-indigo-600 hover:bg-indigo-500 px-4 text-white disabled:opacity-60"
            disabled={checkingLookup}
            onClick={async () => {
              const email = lookupEmail.trim().toLowerCase();
              setLookupError("");
              if (!email) {
                setLookupError("Email is required.");
                return;
              }
              setCheckingLookup(true);
              try {
                const receipt = await checkParticipantStatus(email);
                if (!receipt || receipt.status !== "paid") {
                  setLookupError(
                    "No completed transaction found for this email.",
                  );
                  return;
                }
                hydrateStatus(receipt);
                setStep(2);
                window.setTimeout(() => {
                  step3SectionRef.current?.scrollIntoView({
                    behavior: "smooth",
                    block: "center",
                  });
                  setHighlightStep3(true);
                  window.setTimeout(() => setHighlightStep3(false), 2200);
                }, 80);
              } finally {
                setCheckingLookup(false);
              }
            }}
          >
            {checkingLookup && (
              <div className="absolute inset-0 flex items-center justify-center bg-blue-600/50">
                <svg
                  className="animate-spin h-5 w-5 text-white"
                  xmlns="http://www.w3.org"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
              </div>
            )}

            <span className={`${checkingLookup ? "opacity-20" : ""}`}>
              Check
            </span>
          </button>
        </div>
        {lookupError && (
          <p className="mt-2 text-sm text-rose-600">{lookupError}</p>
        )}
      </section>

      {isCheckoutExpired ? (
        <div className="mt-6 rounded-xl border border-amber-300 bg-amber-50 dark:bg-amber-950/20 p-4 text-sm text-amber-800 dark:text-amber-300">
          This checkout has expired. New payments are disabled, but you can still use
          transaction lookup and receipt display below.
        </div>
      ) : (
        <div className="mt-6 rounded-xl border border-slate-200 dark:border-slate-700 app-surface p-4">
          <Steps
            current={step}
            direction="horizontal"
            items={[
              { title: "Participant Info" },
              { title: "Payment Method" },
              { title: "Pay & Receipt" },
            ]}
          />
        </div>
      )}

      {error && (
        <div className="mt-4 rounded border border-rose-300 bg-rose-50 p-3 text-sm text-rose-700">
          {error}
        </div>
      )}

      {!isCheckoutExpired && step === 0 && (
        <section className="mt-4 rounded-xl border border-slate-200 dark:border-slate-700 app-surface p-4">
          <h2 className="mb-3 text-lg font-semibold">
            Step 1: Participant Form
          </h2>
          <div className="grid gap-3 md:grid-cols-2">
            {(eventData.participantForm || []).map((field, idx) => (
              <label key={`${field.field_name}-${idx}`} className="text-sm">
                {field.field_name}{" "}
                {field.required ? <span className="text-rose-600">*</span> : ""}
                <input
                  className={`mt-1 w-full rounded border px-3 py-2 ${
                    participantErrors[field.field_name] ? "border-rose-500" : ""
                  }`}
                  type="text"
                  value={participantData[field.field_name] || ""}
                  onChange={(e) => {
                    setParticipantData((prev) => ({
                      ...prev,
                      [field.field_name]: e.target.value,
                    }));
                    setParticipantErrors((prev) => {
                      if (!prev[field.field_name]) return prev;
                      const next = { ...prev };
                      delete next[field.field_name];
                      return next;
                    });
                  }}
                />
                {participantErrors[field.field_name] && (
                  <p className="mt-1 text-xs text-rose-600">
                    {participantErrors[field.field_name]}
                  </p>
                )}
              </label>
            ))}
          </div>
          <button
            className="mt-4 rounded bg-indigo-600 hover:bg-indigo-500 px-4 py-2 text-white"
            onClick={async () => {
              if (!validateStep1()) return;
              const email = emailFromParticipantData();
              if (email) {
                const receipt = await checkParticipantStatus(email);
                if (receipt?.status === "paid") {
                  setLookupEmail(email);
                  setLookupError(
                    "This email already has a successful transaction. Please check transaction info in the form above.",
                  );
                  lookupSectionRef.current?.scrollIntoView({
                    behavior: "smooth",
                    block: "center",
                  });
                  setHighlightLookup(true);
                  window.setTimeout(() => setHighlightLookup(false), 2200);
                  return;
                }
              }
              setStep(1);
            }}
          >
            Continue
          </button>
        </section>
      )}

      {!isCheckoutExpired && step === 1 && (
        <section className="mt-4 rounded-xl border border-slate-200 dark:border-slate-700 app-surface p-4">
          <h2 className="mb-3 text-lg font-semibold">Step 2: Payment Method</h2>
          <p className="mb-2 text-sm text-slate-600">
            Deposit Amount: {eventData.amountUsdc} USDC
          </p>
          <Select
            value={paymentMethod}
            className="w-full max-w-xs"
            onChange={(v) => {
              setPaymentMethod(v as PaymentMethod);
              setError("");
            }}
            options={availableMethods}
          />
          <div className="mt-4 flex gap-2">
            <button
              className="rounded border px-4 py-2"
              onClick={() => setStep(0)}
            >
              Back
            </button>
            <button
              className="rounded bg-indigo-600 hover:bg-indigo-500 px-4 py-2 text-white"
              onClick={() => {
                clearPending();
                setStatusData(null);
                setShowFullSignature(false);
                setError("");
                setSessionMethod(paymentMethod);
                setStep(2);
              }}
            >
              Continue
            </button>
          </div>
        </section>
      )}

      {(step === 2 || isCheckoutExpired) && (
        <section
          ref={step3SectionRef}
          className={`mt-4 rounded-xl border p-4 transition-all ${
            highlightStep3 ? "ring-2 ring-amber-400 border-amber-400" : ""
          }`}
        >
          <h2 className="mb-3 text-lg font-semibold">Step 3: Pay & Receipt</h2>
          {!isCheckoutExpired && (
            <p className="mb-3 text-sm text-slate-600">
              Method: {methodLabel(chosenMethod)}
            </p>
          )}

          {!isCheckoutExpired && chosenMethod === "wallet" &&
            reference &&
            timeLeft !== null &&
            !isPaid && (
              <div className="mb-4 rounded border border-amber-300 bg-amber-50 dark:bg-amber-900/30 dark:border-amber-700 p-3 text-sm text-amber-900 dark:text-amber-100">
                <p>
                  Session reference:{" "}
                  <span className="font-mono">{reference}</span>
                </p>
                <p>Time left: {formatSeconds(timeLeft)}</p>
                {timeLeft <= 0 && (
                  <button
                    className="mt-2 rounded border px-3 py-1"
                    onClick={startWalletPayment}
                  >
                    Refresh Session
                  </button>
                )}
              </div>
            )}

          {!isCheckoutExpired && (chosenMethod === "wallet" ? (
            <div className="rounded-xl border border-slate-200 dark:border-slate-700 app-surface p-4 shadow-sm">
              <p className="mb-3 text-sm text-slate-600">
                Connect your wallet and pay your event deposit in one flow.
              </p>
              <div className="flex flex-col gap-3 md:flex-row md:items-center">
                <WalletMultiButton className="!h-11 !rounded-lg !bg-slate-900 !px-4 !text-white" />
                <button
                  className="h-11 rounded-lg bg-emerald-600 px-5 text-sm font-medium text-white disabled:opacity-60"
                  disabled={
                    isPaid ||
                    loadingPay ||
                    !publicKey ||
                    (timeLeft !== null && timeLeft <= 0)
                  }
                  onClick={startWalletPayment}
                >
                  {isPaid
                    ? "Payment Completed"
                    : loadingPay
                      ? "Processing..."
                      : "Deposit Now"}
                </button>
              </div>
            </div>
          ) : (
            <div className="rounded-xl border border-slate-200 dark:border-slate-700 app-surface p-4 shadow-sm">
              <p className="mb-3 text-sm text-slate-600">
                Open the QR payment window, scan with your wallet app, and wait
                for automatic verification.
              </p>
              <button
                className="h-11 rounded-lg bg-indigo-600 hover:bg-indigo-500 px-5 text-sm font-medium text-white disabled:opacity-60"
                disabled={loadingPay}
                onClick={startQrSession}
              >
                {loadingPay ? "Creating session..." : "Open QR Payment Window"}
              </button>
            </div>
          ))}

          {(statusData?.reference ||
            statusData?.status ||
            statusData?.signature) && (
            <div className="mt-4 rounded border border-slate-200 dark:border-slate-700 app-surface p-3 text-sm">
              <p>
                Reference:{" "}
                <span className="font-mono">
                  {statusData?.reference || "-"}
                </span>
              </p>
              <p className="flex items-center gap-2">
                Status:
                <span className={checkoutStatusClass(statusData?.status)}>
                  {(statusData?.status || "pending_payment").replaceAll("_", " ")}
                </span>
              </p>
              <p>
                Method: {methodLabel(statusData?.paymentMethod || chosenMethod)}
              </p>
              {statusData?.signature && (
                <div>
                  <p className="mb-1">Signature:</p>
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs">
                      {statusData.signature.length > 24
                        ? `${statusData.signature.slice(0, 24)}...`
                        : statusData.signature}
                    </span>
                    <button
                      type="button"
                      title={statusData.signature}
                      className="rounded border px-1.5 py-0.5 text-xs"
                      onClick={() => setShowFullSignature((v) => !v)}
                    >
                      i
                    </button>
                    <button
                      type="button"
                      className="rounded border px-1.5 py-0.5 text-xs"
                      onClick={async () => {
                        try {
                          await navigator.clipboard.writeText(
                            statusData.signature,
                          );
                          void message.success("Signature copied to clipboard.");
                        } catch {
                          // no-op
                        }
                      }}
                      title="Copy signature"
                    >
                      ⧉
                    </button>
                  </div>
                  {showFullSignature && (
                    <p className="mt-1 break-all font-mono text-[11px] text-slate-600">
                      {statusData.signature}
                    </p>
                  )}
                </div>
              )}
              {statusData?.solscanUrl && (
                <a
                  className="text-blue-600 underline"
                  target="_blank"
                  href={statusData.solscanUrl}
                  rel="noreferrer"
                >
                  View transaction on Solscan
                </a>
              )}
            </div>
          )}

          {!isCheckoutExpired && !isPaid && (
            <div className="mt-4">
              <button
                className="rounded border px-4 py-2"
                onClick={async () => {
                  if (hasActiveSession) {
                    const ok = window.confirm(
                      "You have an active payment session. Going back will cancel this session. Continue?",
                    );
                    if (!ok) return;
                    await cancelCurrentSession();
                    setStep(1);
                    return;
                  }
                  clearPending();
                  setStatusData(null);
                  setShowFullSignature(false);
                  setError("");
                  setStep(1);
                }}
              >
                Back
              </button>
            </div>
          )}
        </section>
      )}

    </main>
  );
}

export default function CheckoutPage() {
  return <CheckoutInner />;
}
