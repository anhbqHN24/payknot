"use client";

import { useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { useParams } from "next/navigation";
import { useConnection, useWallet } from "@solana/wallet-adapter-react";
import { PublicKey } from "@solana/web3.js";
import { createUsdcTransfer } from "@/lib/solana";

const WalletMultiButton = dynamic(
  () =>
    import("@solana/wallet-adapter-react-ui").then(
      (mod) => mod.WalletMultiButton,
    ),
  { ssr: false },
);

type EventData = {
  slug: string;
  title: string;
  description: string;
  eventImageUrl: string;
  eventDate: string;
  location: string;
  organizerName: string;
  merchantWallet: string;
  amountUsdc: string;
  amountRaw: number;
  network: string;
};

type CheckoutStatus = {
  reference: string;
  status: string;
  signature: string;
  network: string;
  solscanUrl: string;
  approvedBy: string;
  approvedAt: string;
  reason: string;
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

function CheckoutInner() {
  const { slug } = useParams<{ slug: string }>();
  const { connection } = useConnection();
  const { publicKey, sendTransaction } = useWallet();

  const [eventData, setEventData] = useState<EventData | null>(null);
  const [inviteCode, setInviteCode] = useState("");
  const [inviteValid, setInviteValid] = useState<boolean | null>(null);
  const [inviteReason, setInviteReason] = useState("");
  const [existingReceipt, setExistingReceipt] = useState<CheckoutStatus | null>(
    null,
  );
  const [loadingEvent, setLoadingEvent] = useState(true);
  const [loadingPay, setLoadingPay] = useState(false);
  const [reference, setReference] = useState("");
  const [status, setStatus] = useState("");
  const [solscanURL, setSolscanURL] = useState("");
  const [error, setError] = useState("");
  const [timeLeft, setTimeLeft] = useState<number | null>(null);
  const [manualSignature, setManualSignature] = useState("");
  const [txSignature, setTxSignature] = useState("");
  const [walletForManual, setWalletForManual] = useState("");
  const [isMobile, setIsMobile] = useState(false);

  const storageKey = `checkout_pending_${slug}`;

  useEffect(() => {
    (async () => {
      try {
        setLoadingEvent(true);
        const res = await fetch(`/api/checkout/${slug}`);
        if (!res.ok) throw new Error(await res.text());
        const data = (await res.json()) as EventData;
        setEventData(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load event");
      } finally {
        setLoadingEvent(false);
      }
    })();
  }, [slug]);

  useEffect(() => {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return;
    try {
      const data = JSON.parse(raw) as {
        reference: string;
        expiresAt: number;
        signature?: string;
      };
      if (data.reference) setReference(data.reference);
      if (data.signature) {
        setTxSignature(data.signature);
        setManualSignature(data.signature);
      }
      if (data.expiresAt && data.expiresAt > Date.now()) {
        setTimeLeft(Math.floor((data.expiresAt - Date.now()) / 1000));
      }
    } catch {
      localStorage.removeItem(storageKey);
    }
  }, [storageKey]);

  useEffect(() => {
    const update = () => setIsMobile(window.innerWidth < 768);
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, []);

  useEffect(() => {
    if (timeLeft === null) return;
    if (timeLeft <= 0) return;
    const timer = setInterval(() => {
      setTimeLeft((prev) => (prev && prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [timeLeft]);

  const displayDate = useMemo(() => {
    if (!eventData?.eventDate) return "";
    return new Date(eventData.eventDate).toLocaleString();
  }, [eventData]);

  const hasPaidReceipt =
    existingReceipt?.status === "paid" ||
    existingReceipt?.status === "approved" ||
    status === "paid" ||
    status === "approved";

  const persistPending = (ref: string, signature?: string) => {
    const expiresAt = Date.now() + 10 * 60 * 1000;
    setTimeLeft(10 * 60);
    localStorage.setItem(
      storageKey,
      JSON.stringify({ reference: ref, expiresAt, signature: signature || "" }),
    );
  };

  const clearPending = () => {
    localStorage.removeItem(storageKey);
    setTimeLeft(null);
  };

  const hydrateFromReceipt = (receipt: CheckoutStatus) => {
    setExistingReceipt(receipt);
    setReference(receipt?.reference || "");
    setStatus(receipt?.status || "");
    setSolscanURL(receipt?.solscanUrl || "");
    setTxSignature(receipt?.signature || "");
    setManualSignature(receipt?.signature || "");

    if (receipt?.status === "paid" || receipt?.status === "approved") {
      clearPending();
    }
  };

  const validateCode = async () => {
    const code = inviteCode.trim().toUpperCase();
    if (!code) return;

    setError("");
    setInviteReason("");

    const res = await fetch("/api/invite/status", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slug, code }),
    });

    if (!res.ok) {
      setError(await res.text());
      return;
    }

    const data = await res.json();
    hydrateFromReceipt(data.receipt as CheckoutStatus);
    if (data.receipt) {
      setInviteValid(true);
      return;
    }
    setInviteValid(Boolean(data.valid));
    setInviteReason(String(data.reason || ""));

    setExistingReceipt(null);
    if (!data.valid) {
      setReference("");
      setStatus("");
      setSolscanURL("");
    }
  };

  const refreshStatus = async (refArg?: string) => {
    const ref = refArg || reference;
    if (!ref) return;

    const res = await fetch(`/api/checkout/status?reference=${ref}`);
    if (!res.ok) return;
    const data = (await res.json()) as CheckoutStatus;
    hydrateFromReceipt(data);
  };

  const startPayment = async () => {
    if (!publicKey || !eventData) return;

    setLoadingPay(true);
    setError("");

    let createdReference = "";
    let sentSignature = "";
    try {
      const createRes = await fetch("/api/checkout/invoice", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          slug,
          inviteCode: inviteCode.trim().toUpperCase(),
          walletAddress: publicKey.toBase58(),
        }),
      });
      if (!createRes.ok) throw new Error(await createRes.text());

      const { reference: ref } = await createRes.json();
      createdReference = ref;
      setReference(ref);
      setStatus("pending_payment");
      persistPending(ref);
      setWalletForManual(publicKey.toBase58());

      const tx = await createUsdcTransfer(
        connection,
        publicKey,
        new PublicKey(eventData.merchantWallet),
        Number(eventData.amountUsdc),
        ref,
      );

      tx.feePayer = publicKey;
      const latestBlockhash = await connection.getLatestBlockhash("confirmed");
      tx.recentBlockhash = latestBlockhash.blockhash;
      tx.lastValidBlockHeight = latestBlockhash.lastValidBlockHeight;

      const signature = await sendTransaction(tx, connection, {
        skipPreflight: false,
        maxRetries: 5,
      });
      sentSignature = signature;
      setTxSignature(signature);
      setManualSignature(signature);
      persistPending(ref, signature);

      const confirmed = await connection.confirmTransaction(
        {
          signature,
          blockhash: latestBlockhash.blockhash,
          lastValidBlockHeight: latestBlockhash.lastValidBlockHeight,
        },
        "confirmed",
      );
      if (confirmed.value.err) {
        throw new Error("Transaction failed on-chain");
      }

      const confirmRes = await fetch("/api/checkout/confirm", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reference: ref, signature }),
      });
      if (!confirmRes.ok) {
        const text = await confirmRes.text();
        throw new Error(text || "Backend confirmation failed");
      }

      const statusData = (await confirmRes.json()) as CheckoutStatus;
      hydrateFromReceipt(statusData);
      if (!statusData.solscanUrl && eventData.network) {
        setSolscanURL(
          eventData.network === "mainnet"
            ? `https://solscan.io/tx/${signature}`
            : `https://solscan.io/tx/${signature}?cluster=devnet`,
        );
      }
    } catch (err) {
      if (createdReference && !sentSignature) {
        await fetch("/api/checkout/cancel", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reference: createdReference }),
        });
        clearPending();
      }
      setError(err instanceof Error ? err.message : "Payment failed");
    } finally {
      setLoadingPay(false);
    }
  };

  const recheckPayment = async () => {
    if (!reference) {
      setError("No pending reference found.");
      return;
    }

    setError("");
    const res = await fetch("/api/checkout/recheck", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ reference, signature: manualSignature.trim() }),
    });
    if (!res.ok) {
      setError(await res.text());
      return;
    }

    const data = (await res.json()) as CheckoutStatus;
    hydrateFromReceipt(data);
  };

  const manualVerifyPayment = async () => {
    if (!eventData) return;

    if (!manualSignature.trim()) {
      setError("Please provide a transaction signature for manual verify.");
      return;
    }

    const participantWallet = walletForManual || publicKey?.toBase58() || "";
    if (!participantWallet) {
      setError("Please input the wallet address used for transfer.");
      return;
    }

    setError("");
    const res = await fetch("/api/checkout/manual-verify", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        slug,
        inviteCode: inviteCode.trim().toUpperCase(),
        walletAddress: participantWallet,
        signature: manualSignature.trim(),
      }),
    });
    if (!res.ok) {
      setError(await res.text());
      return;
    }

    const data = (await res.json()) as CheckoutStatus;
    hydrateFromReceipt(data);
  };

  if (loadingEvent) {
    return <div className="p-8">Loading event...</div>;
  }
  if (!eventData) {
    return <div className="p-8 text-red-600">Event not found.</div>;
  }

  const getStatusDisplay = () => {
    const currentStatus = status || existingReceipt?.status || "pending";

    switch (currentStatus.toLowerCase()) {
      case "rejected":
        return (
          <span className="text-red-600 uppercase font-semibold">
            {currentStatus}
          </span>
        );
      case "approved":
        return (
          <span className="text-green-600 uppercase font-semibold">
            {currentStatus}
          </span>
        );
      default:
        return <span className="text-black font-bold">{currentStatus}</span>;
    }
  };

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900">
      <div className="mx-auto max-w-3xl px-4 py-10 space-y-6">
        <section className="rounded-2xl bg-white border border-slate-200 p-6 shadow-sm">
          {eventData.eventImageUrl && (
            <img
              src={eventData.eventImageUrl}
              alt={eventData.title}
              className="w-full max-w-sm aspect-square object-cover rounded-xl border mb-4 mx-auto"
            />
          )}
          <h1 className="text-3xl font-bold">{eventData.title}</h1>
          <p className="mt-2 text-slate-600 whitespace-pre-wrap">
            {eventData.description}
          </p>
          <div className="mt-4 pt-2 border-t text-sm text-slate-700 space-y-1">
            {displayDate && (
              <p className="font-bold">Event date: {displayDate}</p>
            )}
            {eventData.location && (
              <div className="font-bold">
                Location:{" "}
                <a
                  href={eventData.location}
                  target="_blank"
                  rel="noreferrer"
                  className="text-blue-600 underline"
                >
                  {eventData.location}
                </a>
              </div>
            )}
            {eventData.organizerName && (
              <p className="font-bold">Organizer: {eventData.organizerName}</p>
            )}
          </div>
          <div className="mt-5 rounded-xl bg-slate-100 p-4">
            <p className="text-sm text-slate-500">Participation deposit</p>
            <p className="text-3xl font-bold">{eventData.amountUsdc} USDC</p>
          </div>
        </section>

        <section className="rounded-2xl bg-white border border-slate-200 p-6 shadow-sm space-y-5">
          <h2 className="text-xl font-semibold">Complete Payment</h2>

          <div className="grid grid-cols-1 gap-4 md:gap-3">
            <div
              className={`rounded-lg border p-3 ${inviteValid ? "border-emerald-400 bg-emerald-50" : "border-slate-200"}`}
            >
              <p className="font-semibold">
                Step 1. Enter invitation code and validate
              </p>
              <div className="mt-2 md:flex gap-2">
                <input
                  value={inviteCode}
                  onChange={(e) => {
                    setInviteCode(e.target.value.toUpperCase());
                    setInviteValid(null);
                  }}
                  className="border rounded-lg px-3 py-2 flex-1 font-mono mb-2"
                  placeholder="ENTER-CODE"
                />
                <button
                  onClick={validateCode}
                  className="rounded-lg border px-3 py-2 font-semibold hover:cursor-pointer"
                >
                  Check code
                </button>
              </div>
              {inviteValid === true && (
                <p className="text-emerald-700 text-sm mt-2">Code is valid.</p>
              )}
              {inviteValid === false && (
                <p className="text-red-600 text-sm mt-2">
                  Invalid code: {inviteReason || "not accepted"}
                </p>
              )}
            </div>

            {!isMobile && !hasPaidReceipt && (
              <div
                className={`rounded-lg border p-3 ${publicKey ? "border-emerald-400 bg-emerald-50" : "border-slate-200"}`}
              >
                <p className="font-semibold">Step 2. Connect wallet</p>
                <div className="mt-2">
                  <WalletMultiButton
                    className="!bg-slate-900 hover:!bg-slate-800 !rounded-lg !h-10"
                    disabled={inviteValid !== true || !inviteCode}
                  />
                </div>
              </div>
            )}

            {!hasPaidReceipt && (
              <div className="rounded-lg border border-slate-200 p-3">
                <p className="font-semibold">Step 3. Make payment</p>
                <div className="mt-2 flex items-center justify-between gap-3 flex-wrap">
                  {!isMobile && (
                    <button
                      onClick={startPayment}
                      disabled={
                        loadingPay ||
                        !publicKey ||
                        inviteValid !== true ||
                        !inviteCode
                      }
                      className="rounded-lg bg-emerald-600 text-white px-5 py-2.5 font-semibold disabled:opacity-60"
                    >
                      {loadingPay ? "Processing..." : "Pay Deposit"}
                    </button>
                  )}

                  {timeLeft !== null && (
                    <span
                      className={`font-mono text-sm px-3 py-1 rounded-full ${timeLeft <= 60 ? "bg-red-100 text-red-700" : "bg-amber-100 text-amber-700"}`}
                    >
                      Session expires in {formatSeconds(timeLeft)}
                    </span>
                  )}
                </div>

                <div className="mt-3 text-center">------ OR ------</div>

                <div className="mt-3 hidden md:grid md:grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium mb-1">
                      Recipient wallet (manual transfer option)
                    </p>
                    <p className="text-xs font-mono break-all rounded bg-slate-100 p-2">
                      {eventData.merchantWallet}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <img
                      alt="Wallet QR"
                      className="h-24 w-24 rounded border"
                      src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(eventData.merchantWallet)}`}
                    />
                    <p className="text-xs text-slate-600">
                      Scan to transfer USDC manually
                    </p>
                  </div>
                </div>

                <div className="mt-3 md:hidden space-y-2">
                  <img
                    alt="Wallet QR"
                    className="h-56 w-56 rounded border mx-auto"
                    src={`https://api.qrserver.com/v1/create-qr-code/?size=400x400&data=${encodeURIComponent(eventData.merchantWallet)}`}
                  />
                  <p className="text-xs text-slate-700 font-mono break-all rounded bg-slate-100 p-2">
                    {eventData.merchantWallet}
                  </p>
                </div>

                <div className="mt-4 border-t pt-3 space-y-2">
                  <p className="text-sm font-medium">
                    Manual transfer verification
                  </p>
                  <input
                    value={walletForManual || publicKey?.toBase58() || ""}
                    onChange={(e) => setWalletForManual(e.target.value.trim())}
                    className="border rounded-lg px-3 py-2 w-full text-sm font-mono"
                    placeholder="Your wallet address used for transfer"
                  />
                  <input
                    value={manualSignature}
                    onChange={(e) => setManualSignature(e.target.value.trim())}
                    className="border rounded-lg px-3 py-2 w-full text-sm font-mono"
                    placeholder="Transaction signature"
                  />
                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={manualVerifyPayment}
                      className="rounded-lg bg-emerald-600 text-white px-3 py-1.5 text-sm"
                    >
                      Verify manual transfer
                    </button>
                    {reference && (
                      <button
                        onClick={recheckPayment}
                        className="rounded-lg border px-3 py-1.5 text-sm"
                      >
                        Recheck existing reference
                      </button>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>
          {error && <p className="text-sm text-red-600">{error}</p>}
          {(reference || existingReceipt) && (
            <p className="text-sm text-blue-600">
              Payment receipt found. Please check below to confirm your receipt
              information
            </p>
          )}
        </section>

        {(reference || existingReceipt) && (
          <section className="rounded-2xl bg-white border border-slate-200 p-6 shadow-sm space-y-3">
            <h3 className="text-lg font-semibold">Payment Receipt</h3>
            <p className="text-sm">
              Reference:{" "}
              <span className="font-mono">
                {reference || existingReceipt?.reference}
              </span>
            </p>
            {/* <p className="text-sm">
              Status:{" "}
              <span className="font-semibold">
                {(status || existingReceipt?.status || "pending") &&
                  status == "rejected" && (
                    <span className="text-red-600 uppercase">{status}</span>
                  )}
              </span>
            </p> */}
            <p className="text-sm">Status: {getStatusDisplay()}</p>
            {(existingReceipt?.reason ||
              (status === "rejected" && existingReceipt?.reason)) && (
              <p className="text-sm text-red-600">
                Reason: {existingReceipt?.reason}
              </p>
            )}
            {solscanURL && (
              <a
                href={solscanURL}
                target="_blank"
                rel="noreferrer"
                className="text-blue-600 underline"
              >
                Open transaction on Solscan
              </a>
            )}

            <label className="block text-sm font-medium">
              Transaction signature (for recheck)
            </label>
            <input
              value={manualSignature}
              onChange={(e) => setManualSignature(e.target.value.trim())}
              className="border rounded-lg px-3 py-2 w-full font-mono text-sm"
              placeholder="Paste transaction signature if needed"
            />

            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => refreshStatus()}
                className="rounded-lg border px-3 py-1.5 text-sm"
              >
                Refresh status
              </button>
              <button
                onClick={recheckPayment}
                className="rounded-lg bg-slate-900 text-white px-3 py-1.5 text-sm"
              >
                Recheck transaction
              </button>
            </div>

            {txSignature && (
              <p className="text-xs text-slate-500">
                Latest tx signature:{" "}
                <span className="font-mono">{txSignature}</span>
              </p>
            )}
          </section>
        )}
      </div>
    </div>
  );
}

export default function CheckoutPage() {
  return <CheckoutInner />;
}
