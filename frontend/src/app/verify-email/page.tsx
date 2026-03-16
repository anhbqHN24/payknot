"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";

function VerifyEmailContent() {
  const params = useSearchParams();
  const token = useMemo(() => params.get("token") || "", [params]);
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const [message, setMessage] = useState("Verifying your email...");

  useEffect(() => {
    if (!token) {
      setStatus("error");
      setMessage("Missing verification token.");
      return;
    }
    void (async () => {
      try {
        const res = await fetch("/api/auth/verify-email", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        });
        if (!res.ok) throw new Error(await res.text());
        const data = await res.json();
        setStatus("success");
        setMessage(data.message || "Email verified successfully. You can now login.");
      } catch (err) {
        setStatus("error");
        setMessage(err instanceof Error ? err.message : "Verification failed");
      }
    })();
  }, [token]);

  return (
    <section className="rounded-2xl app-surface border shadow-sm p-6 space-y-3">
      <h1 className="text-xl font-semibold">Email Verification</h1>
      <p
        className={`text-sm ${
          status === "success"
            ? "text-emerald-700"
            : status === "error"
              ? "text-red-600"
              : "text-slate-600"
        }`}
      >
        {message}
      </p>
      <a href="/app" className="inline-block rounded-lg border px-3 py-1.5 text-sm">
        Back to Login
      </a>
    </section>
  );
}

export default function VerifyEmailPage() {
  return (
    <main className="min-h-screen app-bg">
      <div className="mx-auto max-w-lg px-4 py-14">
        <Suspense
          fallback={
            <section className="rounded-2xl app-surface border shadow-sm p-6">
              <p className="text-sm text-slate-600">Loading verification...</p>
            </section>
          }
        >
          <VerifyEmailContent />
        </Suspense>
      </div>
    </main>
  );
}
