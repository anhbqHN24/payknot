"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";

function ResetPasswordContent() {
  const params = useSearchParams();
  const token = useMemo(() => params.get("token") || "", [params]);
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const [message, setMessage] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!token) {
      setStatus("error");
      setMessage("Missing reset token.");
    }
  }, [token]);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setStatus("idle");
    setMessage("");
    if (!token) {
      setStatus("error");
      setMessage("Missing reset token.");
      return;
    }
    if (password.length < 8) {
      setStatus("error");
      setMessage("Password must be at least 8 characters.");
      return;
    }
    if (password !== confirm) {
      setStatus("error");
      setMessage("Passwords do not match.");
      return;
    }
    setSubmitting(true);
    try {
      const res = await fetch("/api/auth/reset-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, newPassword: password }),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setStatus("success");
      setMessage(data.message || "Password reset successful. You can now login.");
      setPassword("");
      setConfirm("");
    } catch (err) {
      setStatus("error");
      setMessage(err instanceof Error ? err.message : "Password reset failed");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <section className="rounded-2xl app-surface border shadow-sm p-6 space-y-3">
      <h1 className="text-xl font-semibold">Reset Password</h1>
      <form onSubmit={onSubmit} className="space-y-3">
        <label className="text-sm block">
          <span className="block mb-1">New password</span>
          <input
            className="border rounded-lg px-3 py-2 w-full"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        <label className="text-sm block">
          <span className="block mb-1">Confirm password</span>
          <input
            className="border rounded-lg px-3 py-2 w-full"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
          />
        </label>
        <button
          disabled={submitting}
          className="w-full rounded-lg bg-slate-900 text-white py-2.5 font-semibold disabled:opacity-60"
        >
          {submitting ? "Please wait..." : "Reset Password"}
        </button>
      </form>
      {message && (
        <p
          className={`text-sm ${
            status === "success" ? "text-emerald-700" : "text-red-600"
          }`}
        >
          {message}
        </p>
      )}
      <a href="/app" className="inline-block rounded-lg border px-3 py-1.5 text-sm">
        Back to Login
      </a>
    </section>
  );
}

export default function ResetPasswordPage() {
  return (
    <main className="min-h-screen app-bg">
      <div className="mx-auto max-w-lg px-4 py-14">
        <Suspense
          fallback={
            <section className="rounded-2xl app-surface border shadow-sm p-6">
              <p className="text-sm text-slate-600">Loading reset form...</p>
            </section>
          }
        >
          <ResetPasswordContent />
        </Suspense>
      </div>
    </main>
  );
}
