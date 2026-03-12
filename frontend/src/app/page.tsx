"use client";

import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import Image from "next/image";
import ImgCrop from "antd-img-crop";
import { Upload, message } from "antd";
import type { UploadFile, UploadProps } from "antd/es/upload/interface";

type AppUser = {
  id: number;
  name: string;
  email: string;
  provider: "password" | "google";
};

type CreateEventResponse = {
  eventId: number;
  slug: string;
  checkoutUrl: string;
  eventImageUrl: string;
  checkoutExpiresAt: string;
  amountUsdc: string;
  merchantWallet: string;
  eventSource: "luma" | "custom";
  sourceUrl: string;
  participantFormSchema: CheckoutField[];
  paymentMethods: {
    wallet: boolean;
    qr: boolean;
  };
};

type EventSummary = {
  eventId: number;
  slug: string;
  title: string;
  description: string;
  eventImageUrl: string;
  eventDate: string;
  checkoutExpiresAt: string;
  location: string;
  organizerName: string;
  merchantWallet: string;
  amountUsdc: string;
  eventSource: "luma" | "custom";
  sourceUrl: string;
  participantFormSchema: CheckoutField[];
  paymentMethods: {
    wallet: boolean;
    qr: boolean;
  };
  createdAt: string;
};

type EventCheckoutRow = {
  id: number;
  walletAddress: string;
  reference: string;
  signature: string;
  status: string;
  createdAt: string;
  paidAt?: string;
  participantData?: Record<string, string>;
};

type CheckoutField = {
  field_name: string;
  required: boolean;
};

type EventSortField = "title" | "organizerName" | "eventDate" | "amountUsdc";
type DetailTab = "info" | "checkoutForm" | "deposits";

const EVENT_PAGE_SIZE = 10;

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (args: {
            client_id: string;
            callback: (resp: { credential: string }) => void;
          }) => void;
          renderButton: (
            parent: HTMLElement,
            options: Record<string, string | number>,
          ) => void;
        };
      };
    };
  }
}

function toBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      resolve(typeof reader.result === "string" ? reader.result : "");
    };
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
}

function toLocalDateTimeInput(value: string) {
  if (!value) return "";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  const yyyy = d.getFullYear();
  const mm = pad(d.getMonth() + 1);
  const dd = pad(d.getDate());
  const hh = pad(d.getHours());
  const mi = pad(d.getMinutes());
  return `${yyyy}-${mm}-${dd}T${hh}:${mi}`;
}

function datetimeLocalToRFC3339Local(value: string) {
  if (!value) return "";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  const yyyy = d.getFullYear();
  const mm = pad(d.getMonth() + 1);
  const dd = pad(d.getDate());
  const hh = pad(d.getHours());
  const mi = pad(d.getMinutes());
  const ss = "00";
  const offsetMin = -d.getTimezoneOffset();
  const sign = offsetMin >= 0 ? "+" : "-";
  const abs = Math.abs(offsetMin);
  const oh = pad(Math.floor(abs / 60));
  const om = pad(abs % 60);
  return `${yyyy}-${mm}-${dd}T${hh}:${mi}:${ss}${sign}${oh}:${om}`;
}

function sanitizeRichHtml(input: string) {
  if (!input) return "";
  return input
    .replace(/<script[\s\S]*?>[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?>[\s\S]*?<\/style>/gi, "")
    .replace(/\son\w+="[^"]*"/gi, "")
    .replace(/\son\w+='[^']*'/gi, "");
}

export default function Home() {
  const googleInitializedRef = useRef(false);
  const detailCloseTimerRef = useRef<number | null>(null);
  const pageHeaderRef = useRef<HTMLElement | null>(null);
  const sessionsHeaderRef = useRef<HTMLDivElement | null>(null);
  const [googleScriptReady, setGoogleScriptReady] = useState(false);
  const [currentUser, setCurrentUser] = useState<AppUser | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [authSubmitting, setAuthSubmitting] = useState(false);
  const [authMode, setAuthMode] = useState<"login" | "register">("login");
  const [authName, setAuthName] = useState("");
  const [authEmail, setAuthEmail] = useState("");
  const [authPassword, setAuthPassword] = useState("");

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [eventDate, setEventDate] = useState("");
  const [checkoutExpiresAt, setCheckoutExpiresAt] = useState("");
  const [location, setLocation] = useState("");
  const [organizerName, setOrganizerName] = useState("");
  const [merchantWallet, setMerchantWallet] = useState("");
  const [amountUsdc, setAmountUsdc] = useState("10");
  const [eventImageUrl, setEventImageUrl] = useState("");
  const [eventSource, setEventSource] = useState<"luma" | "custom">("custom");
  const [sourceUrl, setSourceURL] = useState("");
  const [importingLuma, setImportingLuma] = useState(false);
  const [importedLumaImageURL, setImportedLumaImageURL] = useState("");
  const uploadTriggerRef = useRef<HTMLDivElement | null>(null);
  const descriptionEditorRef = useRef<HTMLDivElement | null>(null);
  const [participantFields, setParticipantFields] = useState<CheckoutField[]>([
    { field_name: "name", required: true },
    { field_name: "email", required: true },
  ]);
  const [imageFileList, setImageFileList] = useState<UploadFile[]>([]);

  const [events, setEvents] = useState<EventSummary[]>([]);
  const [selectedEventID, setSelectedEventID] = useState<number | null>(null);
  const [detailRender, setDetailRender] = useState(false);
  const [detailClosing, setDetailClosing] = useState(false);
  const [detailMode, setDetailMode] = useState<"create" | "edit">("create");
  const [detailTab, setDetailTab] = useState<DetailTab>("info");
  const [showCheckoutFormSection, setShowCheckoutFormSection] = useState(true);
  const [showDepositsSection, setShowDepositsSection] = useState(true);
  const [checkoutFormAccordionOpen, setCheckoutFormAccordionOpen] =
    useState(true);
  const [depositAccordionOpen, setDepositAccordionOpen] = useState(true);
  const [checkoutFormSearchTerm, setCheckoutFormSearchTerm] = useState("");
  const [checkoutFormPage, setCheckoutFormPage] = useState(1);
  const [depositSearchTerm, setDepositSearchTerm] = useState("");
  const [depositPage, setDepositPage] = useState(1);
  const [isMobile, setIsMobile] = useState(false);
  const [desktopHeaderHeight, setDesktopHeaderHeight] = useState(0);
  const [sessionsHeaderPinned, setSessionsHeaderPinned] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [eventDateFrom, setEventDateFrom] = useState("");
  const [eventDateTo, setEventDateTo] = useState("");
  const [sortField, setSortField] = useState<EventSortField>("eventDate");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");
  const [eventPage, setEventPage] = useState(1);
  const [creating, setCreating] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");
  const [eventFormErrors, setEventFormErrors] = useState<
    Record<string, string>
  >({});
  const [loadingEvents, setLoadingEvents] = useState(false);

  const [checkouts, setCheckouts] = useState<EventCheckoutRow[]>([]);
  const [loadingCheckouts, setLoadingCheckouts] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch("/api/auth/me", { credentials: "include" });
        if (!res.ok) {
          setCurrentUser(null);
          return;
        }
        const data = await res.json();
        setCurrentUser(data.user as AppUser);
      } catch {
        setCurrentUser(null);
      } finally {
        setAuthLoading(false);
      }
    })();
  }, []);

  useEffect(() => {
    if (authLoading || currentUser) return;

    const clientID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID;
    if (!clientID) return;

    if (window.google) {
      setGoogleScriptReady(true);
      return;
    }

    const existing = document.getElementById(
      "google-identity-script",
    ) as HTMLScriptElement | null;
    const onLoad = () => setGoogleScriptReady(true);

    if (existing) {
      existing.addEventListener("load", onLoad);
      return () => existing.removeEventListener("load", onLoad);
    }

    const script = document.createElement("script");
    script.id = "google-identity-script";
    script.src = "https://accounts.google.com/gsi/client";
    script.async = true;
    script.defer = true;
    script.addEventListener("load", onLoad);
    document.body.appendChild(script);

    return () => script.removeEventListener("load", onLoad);
  }, [authLoading, currentUser]);

  useEffect(() => {
    if (authLoading || currentUser || !googleScriptReady) return;

    const clientID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID;
    if (!clientID || !window.google) return;

    if (!googleInitializedRef.current) {
      window.google.accounts.id.initialize({
        client_id: clientID,
        callback: async (resp: { credential: string }) => {
          try {
            setAuthSubmitting(true);
            const loginRes = await fetch("/api/auth/google", {
              method: "POST",
              credentials: "include",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ credential: resp.credential }),
            });
            if (!loginRes.ok) throw new Error(await loginRes.text());
            const data = await loginRes.json();
            setCurrentUser(data.user as AppUser);
            setError("");
          } catch (err) {
            setError(
              err instanceof Error ? err.message : "Google sign-in failed",
            );
          } finally {
            setAuthSubmitting(false);
          }
        },
      });
      googleInitializedRef.current = true;
    }

    const render = () => {
      const container = document.getElementById("google-login-button");
      if (!container || !window.google) return;
      container.innerHTML = "";
      window.google.accounts.id.renderButton(container, {
        theme: "outline",
        size: "large",
        shape: "rectangular",
        width: 300,
        text: "continue_with",
      });
    };

    render();
    const timer = window.setTimeout(render, 120);
    return () => window.clearTimeout(timer);
  }, [authLoading, currentUser, googleScriptReady, authMode]);

  const selectedEvent = useMemo(
    () => events.find((e) => e.eventId === selectedEventID) || null,
    [events, selectedEventID],
  );
  const hasPaidDeposits = useMemo(
    () => checkouts.some((c) => c.status === "paid"),
    [checkouts],
  );

  const filteredAndSortedEvents = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    const from = eventDateFrom ? new Date(eventDateFrom).getTime() : null;
    const to = eventDateTo ? new Date(eventDateTo).getTime() : null;

    const filtered = events.filter((ev) => {
      const haystack = [
        ev.title,
        ev.organizerName,
        ev.description,
        ev.merchantWallet,
        ev.location,
      ]
        .join(" ")
        .toLowerCase();
      if (q && !haystack.includes(q)) return false;

      if (from !== null || to !== null) {
        if (!ev.eventDate) return false;
        const t = new Date(ev.eventDate).getTime();
        if (Number.isNaN(t)) return false;
        if (from !== null && t < from) return false;
        if (to !== null && t > to + 24 * 60 * 60 * 1000 - 1) return false;
      }
      return true;
    });

    const sorted = [...filtered].sort((a, b) => {
      const direction = sortDirection === "asc" ? 1 : -1;
      if (sortField === "title") {
        return a.title.localeCompare(b.title) * direction;
      }
      if (sortField === "organizerName") {
        return (
          (a.organizerName || "").localeCompare(b.organizerName || "") *
          direction
        );
      }
      if (sortField === "eventDate") {
        const aT = a.eventDate ? new Date(a.eventDate).getTime() : 0;
        const bT = b.eventDate ? new Date(b.eventDate).getTime() : 0;
        return (aT - bT) * direction;
      }
      const aAmount = Number.parseFloat(a.amountUsdc || "0");
      const bAmount = Number.parseFloat(b.amountUsdc || "0");
      return (aAmount - bAmount) * direction;
    });

    return sorted;
  }, [
    events,
    searchQuery,
    eventDateFrom,
    eventDateTo,
    sortField,
    sortDirection,
  ]);

  const eventTotalPages = useMemo(
    () =>
      Math.max(1, Math.ceil(filteredAndSortedEvents.length / EVENT_PAGE_SIZE)),
    [filteredAndSortedEvents.length],
  );

  const paginatedEvents = useMemo(() => {
    const start = (eventPage - 1) * EVENT_PAGE_SIZE;
    return filteredAndSortedEvents.slice(start, start + EVENT_PAGE_SIZE);
  }, [filteredAndSortedEvents, eventPage]);

  const fullCheckoutLink = useMemo(() => {
    if (!selectedEvent) return "";
    if (typeof window === "undefined") return `/checkout/${selectedEvent.slug}`;
    return `${window.location.origin}/checkout/${selectedEvent.slug}`;
  }, [selectedEvent]);

  const filteredCheckoutFields = useMemo(() => {
    const q = checkoutFormSearchTerm.trim().toLowerCase();
    return participantFields.filter((field) => {
      const haystack = `${field.field_name}`.toLowerCase();
      return q ? haystack.includes(q) : true;
    });
  }, [participantFields, checkoutFormSearchTerm]);

  const checkoutFormTotalPages = useMemo(
    () => Math.max(1, Math.ceil(filteredCheckoutFields.length / 10)),
    [filteredCheckoutFields.length],
  );
  const pagedCheckoutFields = useMemo(() => {
    const start = (checkoutFormPage - 1) * 10;
    return filteredCheckoutFields.slice(start, start + 10);
  }, [filteredCheckoutFields, checkoutFormPage]);

  const filteredDeposits = useMemo(() => {
    const q = depositSearchTerm.trim().toLowerCase();
    return checkouts.filter((row) => {
      const haystack = [
        row.walletAddress,
        row.reference,
        row.signature,
        row.status,
        JSON.stringify(row.participantData || {}),
      ]
        .join(" ")
        .toLowerCase();
      return q ? haystack.includes(q) : true;
    });
  }, [checkouts, depositSearchTerm]);

  const depositTotalPages = useMemo(
    () => Math.max(1, Math.ceil(filteredDeposits.length / 10)),
    [filteredDeposits.length],
  );
  const pagedDeposits = useMemo(() => {
    const start = (depositPage - 1) * 10;
    return filteredDeposits.slice(start, start + 10);
  }, [filteredDeposits, depositPage]);

  useEffect(() => {
    if (!currentUser) return;
    void loadEvents();
  }, [currentUser]);

  useEffect(() => {
    if (!selectedEvent) return;
    fillFormFromEvent(selectedEvent);
    setCheckouts([]);
    void refreshCheckouts(selectedEvent.eventId);
  }, [selectedEventID]);

  useEffect(() => {
    const update = () => setIsMobile(window.innerWidth < 768);
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, []);

  useEffect(() => {
    const measure = () => {
      const h = pageHeaderRef.current?.offsetHeight ?? 0;
      setDesktopHeaderHeight(h);
    };
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, []);

  useEffect(() => {
    const onScroll = () => {
      if (isMobile) {
        setSessionsHeaderPinned(false);
        return;
      }
      if (!sessionsHeaderRef.current) return;
      const rect = sessionsHeaderRef.current.getBoundingClientRect();
      setSessionsHeaderPinned(rect.top <= desktopHeaderHeight + 8);
    };

    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, [isMobile, desktopHeaderHeight]);

  useEffect(() => {
    setCheckoutFormPage(1);
  }, [checkoutFormSearchTerm, selectedEventID, participantFields.length]);

  useEffect(() => {
    if (checkoutFormPage > checkoutFormTotalPages) {
      setCheckoutFormPage(checkoutFormTotalPages);
    }
  }, [checkoutFormPage, checkoutFormTotalPages]);

  useEffect(() => {
    setDepositPage(1);
  }, [depositSearchTerm, selectedEventID, checkouts.length]);

  useEffect(() => {
    if (depositPage > depositTotalPages) setDepositPage(depositTotalPages);
  }, [depositPage, depositTotalPages]);

  useEffect(() => {
    setEventPage(1);
  }, [searchQuery, eventDateFrom, eventDateTo, sortField, sortDirection]);

  useEffect(() => {
    if (eventPage > eventTotalPages) {
      setEventPage(eventTotalPages);
    }
  }, [eventPage, eventTotalPages]);

  const fillFormFromEvent = (ev: EventSummary) => {
    setTitle(ev.title);
    setDescription(ev.description || "");
    setEventDate(ev.eventDate ? toLocalDateTimeInput(ev.eventDate) : "");
    setCheckoutExpiresAt(
      ev.checkoutExpiresAt ? toLocalDateTimeInput(ev.checkoutExpiresAt) : "",
    );
    setLocation(ev.location || "");
    setOrganizerName(ev.organizerName || "");
    setMerchantWallet(ev.merchantWallet || "");
    setAmountUsdc(ev.amountUsdc || "10");
    setEventImageUrl(ev.eventImageUrl || "");
    setEventSource(ev.eventSource || "custom");
    setSourceURL(ev.sourceUrl || "");
    setParticipantFields(
      ev.participantFormSchema?.length
        ? ev.participantFormSchema
        : [
            { field_name: "name", required: true },
            { field_name: "email", required: true },
          ],
    );
    setImportedLumaImageURL("");
    setImageFileList([]);
  };

  const resetForm = () => {
    setTitle("");
    setDescription("");
    setEventDate("");
    setCheckoutExpiresAt("");
    setLocation("");
    setOrganizerName("");
    setMerchantWallet("");
    setAmountUsdc("10");
    setEventImageUrl("");
    setEventSource("custom");
    setSourceURL("");
    setImportedLumaImageURL("");
    setParticipantFields([
      { field_name: "name", required: true },
      { field_name: "email", required: true },
    ]);
    setImageFileList([]);
    setEventFormErrors({});
  };

  const clearEventFormError = (field: string) => {
    setEventFormErrors((prev) => {
      if (!prev[field]) return prev;
      const next = { ...prev };
      delete next[field];
      return next;
    });
  };

  const loadEvents = async () => {
    setLoadingEvents(true);
    setError("");
    try {
      const res = await fetch(`/api/events`, { credentials: "include" });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      const fetched = (data.events ?? []) as EventSummary[];
      setEvents(fetched);
      setSelectedEventID((prev) =>
        fetched.some((e) => e.eventId === prev) ? prev : null,
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load events");
    } finally {
      setLoadingEvents(false);
    }
  };

  const createEvent = async () => {
    if (!currentUser) {
      setError("Please login first.");
      void message.error("Please login first.");
      return;
    }

    const nextDescription = sanitizeRichHtml(description);
    setError("");
    setCreating(true);
    try {
      const res = await fetch("/api/events", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title,
          description: nextDescription,
          eventImageUrl,
          eventDate: eventDate ? datetimeLocalToRFC3339Local(eventDate) : "",
          checkoutExpiresAt: datetimeLocalToRFC3339Local(checkoutExpiresAt),
          location,
          organizerName,
          merchantWallet,
          amountUsdc: Number(amountUsdc),
          eventSource,
          sourceUrl,
          participantFormSchema: participantFields,
          paymentMethods: { wallet: true, qr: true },
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = (await res.json()) as CreateEventResponse;
      await loadEvents();
      setSelectedEventID(data.eventId);
      setDetailMode("edit");
      void message.success("Event created successfully.");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to create event";
      setError(msg);
      void message.error(msg);
    } finally {
      setCreating(false);
    }
  };

  const updateEvent = async () => {
    if (!currentUser || !selectedEvent) return;
    const nextDescription = sanitizeRichHtml(description);
    setUpdating(true);
    setError("");
    try {
      const res = await fetch(`/api/events/${selectedEvent.eventId}`, {
        method: "PUT",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title,
          description: nextDescription,
          eventImageUrl,
          eventDate: eventDate ? datetimeLocalToRFC3339Local(eventDate) : "",
          checkoutExpiresAt: datetimeLocalToRFC3339Local(checkoutExpiresAt),
          location,
          organizerName,
          merchantWallet,
          amountUsdc: Number(amountUsdc),
          eventSource,
          sourceUrl,
          participantFormSchema: participantFields,
          paymentMethods: { wallet: true, qr: true },
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      await loadEvents();
      void message.success("Event updated successfully.");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to update event";
      setError(msg);
      void message.error(msg);
    } finally {
      setUpdating(false);
    }
  };

  const validateEventForm = () => {
    const nextErrors: Record<string, string> = {};
    if (!eventSource.trim()) {
      nextErrors.eventSource = "Event source is required.";
    }
    if (!title.trim()) {
      nextErrors.title = "Event title is required.";
    }
    if (eventSource === "luma" && !sourceUrl.trim()) {
      nextErrors.sourceUrl =
        "Luma event URL is required when event source is Luma.com.";
    }
    if (eventSource === "luma" && importedLumaImageURL && !eventImageUrl) {
      nextErrors.eventImageUrl =
        "Please upload and crop the imported Luma image before saving.";
    }
    if (!checkoutExpiresAt.trim()) {
      nextErrors.checkoutExpiresAt = "Checkout expires at is required.";
    }
    if (!merchantWallet.trim()) {
      nextErrors.merchantWallet = "Merchant wallet address is required.";
    }
    if (!amountUsdc.trim() || Number(amountUsdc) <= 0) {
      nextErrors.amountUsdc =
        "USDC amount is required and must be greater than 0.";
    }

    setEventFormErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      const msg = Object.values(nextErrors)[0];
      setError(msg);
      void message.error(msg);
      return false;
    }
    setError("");
    return true;
  };

  const saveEventChanges = async () => {
    if (detailMode === "edit" && hasPaidDeposits) {
      const msg =
        "This event has successful participant deposits and can no longer be edited.";
      setError(msg);
      void message.error(msg);
      return;
    }
    if (!validateEventForm()) return;
    if (detailMode === "create") {
      await createEvent();
      return;
    }
    await updateEvent();
  };

  const deleteEvent = async () => {
    if (!currentUser || !selectedEvent) return;
    if (hasPaidDeposits) {
      const msg =
        "This event has successful participant deposits and can no longer be deleted.";
      setError(msg);
      void message.error(msg);
      return;
    }
    const ok = window.confirm(`Delete event \"${selectedEvent.title}\"?`);
    if (!ok) return;

    setDeleting(true);
    setError("");
    try {
      const res = await fetch(`/api/events/${selectedEvent.eventId}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (!res.ok) throw new Error(await res.text());
      await loadEvents();
      setCheckouts([]);
      if (selectedEventID === selectedEvent.eventId) {
        setSelectedEventID(null);
        resetForm();
      }
      void message.success("Event deleted successfully.");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to delete event";
      setError(msg);
      void message.error(msg);
    } finally {
      setDeleting(false);
    }
  };

  const refreshCheckouts = async (eventID?: number) => {
    const id = eventID ?? selectedEvent?.eventId;
    if (!id) return;
    setLoadingCheckouts(true);
    try {
      const res = await fetch(`/api/events/${id}/checkouts`, {
        credentials: "include",
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setCheckouts(data.checkouts ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load checkouts");
    } finally {
      setLoadingCheckouts(false);
    }
  };

  const importLumaEvent = async () => {
    if (eventSource !== "luma" || !sourceUrl.trim()) return;
    setImportingLuma(true);
    setError("");
    try {
      const res = await fetch("/api/events/import/luma", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: sourceUrl.trim() }),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      if (data.title) setTitle(data.title);
      if (data.descriptionHtml) {
        setDescription(sanitizeRichHtml(data.descriptionHtml));
      } else if (data.description) {
        setDescription(sanitizeRichHtml(data.description));
      }
      if (data.eventDate) {
        setEventDate(toLocalDateTimeInput(data.eventDate));
      }
      if (data.location) setLocation(data.location);
      if (data.organizerName) setOrganizerName(data.organizerName);
      if (data.eventImageUrl) {
        setImportedLumaImageURL(data.eventImageUrl);
        setEventImageUrl("");
        setImageFileList([]);
        try {
          const imageRes = await fetch(data.eventImageUrl);
          if (!imageRes.ok) throw new Error("Unable to load imported image");
          const blob = await imageRes.blob();
          const mime = blob.type || "image/jpeg";
          const ext =
            mime === "image/png"
              ? "png"
              : mime === "image/webp"
                ? "webp"
                : "jpg";
          const importedFile = new File([blob], `luma-import.${ext}`, {
            type: mime,
          });
          const input = uploadTriggerRef.current?.querySelector(
            "input[type='file']",
          ) as HTMLInputElement | null;
          if (!input) throw new Error("Upload input unavailable");
          const dt = new DataTransfer();
          dt.items.add(importedFile);
          Object.defineProperty(input, "files", {
            configurable: true,
            value: dt.files,
          });
          input.dispatchEvent(new Event("change", { bubbles: true }));
          void message.success("Luma image imported. Crop modal opened.");
        } catch {
          void message.info(
            "Luma image imported. Click upload area to crop before saving.",
          );
        }
      }
      if (data.warning) setError(data.warning);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to import Luma event. If private, set event to public.",
      );
    } finally {
      setImportingLuma(false);
    }
  };

  useEffect(() => {
    if (!descriptionEditorRef.current) return;
    if (descriptionEditorRef.current.innerHTML !== description) {
      descriptionEditorRef.current.innerHTML = description;
    }
  }, [description]);

  const applyDescriptionCommand = (command: string) => {
    if (!descriptionEditorRef.current) return;
    descriptionEditorRef.current.focus();
    if (command === "createLink") {
      const href = window.prompt("Enter URL");
      if (!href) return;
      document.execCommand("createLink", false, href);
    } else {
      document.execCommand(command, false);
    }
    setDescription(sanitizeRichHtml(descriptionEditorRef.current.innerHTML));
  };

  const addParticipantField = () => {
    setParticipantFields((prev) => [
      ...prev,
      { field_name: "", required: false },
    ]);
  };

  const copyText = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      void message.success("Copied");
    } catch {
      setError("Failed to copy text");
    }
  };

  const uploadProps: UploadProps = {
    accept: "image/jpeg,image/png,image/webp",
    fileList: imageFileList,
    maxCount: 1,
    beforeUpload: (file) => {
      const isValidType = ["image/jpeg", "image/png", "image/webp"].includes(
        file.type,
      );
      if (!isValidType) {
        void message.error("Image format must be JPG, PNG, or WEBP");
        return Upload.LIST_IGNORE;
      }
      if (file.size > 2 * 1024 * 1024) {
        void message.error("Image must be <= 2MB");
        return Upload.LIST_IGNORE;
      }
      // Must return the file so antd-img-crop can pass the cropped output through.
      return file;
    },
    onChange: async ({ fileList: list }) => {
      const latest = list.at(-1);
      const nextList = latest ? [latest] : [];
      setImageFileList(nextList);
      const target = (latest?.originFileObj ??
        (latest as unknown as File | undefined)) as File | undefined;
      if (!target) {
        setEventImageUrl("");
        return;
      }
      const b64 = await toBase64(target);
      setEventImageUrl(b64);
      setImportedLumaImageURL("");
      clearEventFormError("eventImageUrl");
    },
    onRemove: () => {
      setImageFileList([]);
      setEventImageUrl("");
    },
    customRequest: ({ onSuccess }) => {
      onSuccess?.("ok");
    },
  };

  const onAuthSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");

    const email = authEmail.trim().toLowerCase();
    const password = authPassword;
    const name = authName.trim();

    if (!email || !password) {
      setError("Email and password are required");
      return;
    }
    if (authMode === "register" && !name) {
      setError("Name is required for register");
      return;
    }

    setAuthSubmitting(true);
    try {
      const endpoint =
        authMode === "register" ? "/api/auth/register" : "/api/auth/login";
      const payload =
        authMode === "register"
          ? { name, email, password }
          : { email, password };

      const res = await fetch(endpoint, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setCurrentUser(data.user as AppUser);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Authentication failed");
    } finally {
      setAuthSubmitting(false);
    }
  };

  const logout = async () => {
    try {
      await fetch("/api/auth/logout", {
        method: "POST",
        credentials: "include",
      });
    } catch {
      // Ignore network errors on logout; clear local state anyway.
    }
    setCurrentUser(null);
    setEvents([]);
    setSelectedEventID(null);
    setCheckouts([]);
    resetForm();
  };

  const openCreateDetail = () => {
    if (detailCloseTimerRef.current) {
      window.clearTimeout(detailCloseTimerRef.current);
      detailCloseTimerRef.current = null;
    }
    setDetailMode("create");
    setSelectedEventID(null);
    resetForm();
    setCheckouts([]);
    setDetailTab("info");
    setDetailRender(true);
    setDetailClosing(false);
  };

  const openEditDetail = (eventID: number) => {
    if (detailCloseTimerRef.current) {
      window.clearTimeout(detailCloseTimerRef.current);
      detailCloseTimerRef.current = null;
    }
    setDetailMode("edit");
    setSelectedEventID(eventID);
    setDetailTab("info");
    setDetailRender(true);
    setDetailClosing(false);
  };

  const closeDetail = () => {
    setDetailClosing(true);
    if (detailCloseTimerRef.current) {
      window.clearTimeout(detailCloseTimerRef.current);
    }
    detailCloseTimerRef.current = window.setTimeout(() => {
      setDetailRender(false);
      setDetailClosing(false);
      detailCloseTimerRef.current = null;
    }, 260);
  };

  useEffect(() => {
    return () => {
      if (detailCloseTimerRef.current) {
        window.clearTimeout(detailCloseTimerRef.current);
      }
    };
  }, []);

  const renderGeneralInfoForm = () => (
    <div className="space-y-4">
      <section className="rounded-lg border border-slate-200 bg-slate-50 p-3 space-y-3">
        <h4 className="text-sm font-semibold text-slate-700">Source</h4>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <label className={`text-sm ${eventSource === "custom" ? "md:col-span-2" : ""}`}>
            <span className="block mb-1 font-medium">
              Event source <span className="text-rose-600">*</span>
            </span>
            <select
              className={`border rounded-lg px-3 py-2 w-full ${
                eventFormErrors.eventSource ? "border-rose-500" : ""
              }`}
              value={eventSource}
              onChange={(e) => {
                const nextSource = e.target.value as "luma" | "custom";
                setEventSource(nextSource);
                if (nextSource !== "luma") {
                  setImportedLumaImageURL("");
                  clearEventFormError("eventImageUrl");
                }
                clearEventFormError("eventSource");
                clearEventFormError("sourceUrl");
              }}
            >
              <option value="custom">Custom</option>
              <option value="luma">Luma.com</option>
            </select>
            {eventFormErrors.eventSource && (
              <p className="mt-1 text-xs text-rose-600">{eventFormErrors.eventSource}</p>
            )}
          </label>
          {eventSource === "luma" && (
            <label className="text-sm md:col-span-2">
              <span className="block mb-1 font-medium">
                Luma event URL <span className="text-rose-600">*</span>
              </span>
              <div className="flex gap-2">
                <input
                  className={`border rounded-lg px-3 py-2 w-full disabled:bg-slate-100 ${
                    eventFormErrors.sourceUrl ? "border-rose-500" : ""
                  }`}
                  value={sourceUrl}
                  onChange={(e) => {
                    setSourceURL(e.target.value);
                    clearEventFormError("sourceUrl");
                  }}
                  placeholder="https://lu.ma/..."
                />
                <button
                  onClick={importLumaEvent}
                  disabled={importingLuma}
                  className="rounded-lg bg-slate-900 text-white px-3 py-2 text-sm disabled:opacity-60"
                >
                  {importingLuma ? "..." : "Import"}
                </button>
              </div>
              {eventFormErrors.sourceUrl && (
                <p className="mt-1 text-xs text-rose-600">{eventFormErrors.sourceUrl}</p>
              )}
            </label>
          )}
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 p-3 space-y-3">
        <h4 className="text-sm font-semibold text-slate-700">General info</h4>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <label className="text-sm md:col-span-2">
            <span className="block mb-1 font-medium">
              Event title <span className="text-rose-600">*</span>
            </span>
            <input
              className={`border rounded-lg px-3 py-2 w-full ${
                eventFormErrors.title ? "border-rose-500" : ""
              }`}
              value={title}
              onChange={(e) => {
                setTitle(e.target.value);
                clearEventFormError("title");
              }}
              required
            />
            {eventFormErrors.title && (
              <p className="mt-1 text-xs text-rose-600">{eventFormErrors.title}</p>
            )}
          </label>
          <label className="text-sm">
            <span className="block mb-1 font-medium">Organizer name</span>
            <input
              className="border rounded-lg px-3 py-2 w-full"
              value={organizerName}
              onChange={(e) => setOrganizerName(e.target.value)}
            />
          </label>
          <label className="text-sm">
            <span className="block mb-1 font-medium">Location link</span>
            <input
              className="border rounded-lg px-3 py-2 w-full"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
            />
          </label>
          <label className="text-sm">
            <span className="block mb-1 font-medium">Event date & time</span>
            <input
              className="border rounded-lg px-3 py-2 w-full"
              type="datetime-local"
              value={eventDate}
              onChange={(e) => setEventDate(e.target.value)}
            />
          </label>
          <label className="text-sm">
            <span className="block mb-1 font-medium">
              Checkout expires at <span className="text-rose-600">*</span>
            </span>
            <input
              className={`border rounded-lg px-3 py-2 w-full ${
                eventFormErrors.checkoutExpiresAt ? "border-rose-500" : ""
              }`}
              type="datetime-local"
              value={checkoutExpiresAt}
              onChange={(e) => {
                setCheckoutExpiresAt(e.target.value);
                clearEventFormError("checkoutExpiresAt");
              }}
            />
            {eventFormErrors.checkoutExpiresAt && (
              <p className="mt-1 text-xs text-rose-600">{eventFormErrors.checkoutExpiresAt}</p>
            )}
          </label>
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 p-3 space-y-3">
        <h4 className="text-sm font-semibold text-slate-700">Description</h4>
        <label className="text-sm block">
          <span className="block mb-1 font-medium">Brief event description</span>
          <div className="rounded-lg border border-slate-300 bg-white">
            <div className="flex flex-wrap gap-1 border-b border-slate-200 px-2 py-1.5">
              <button type="button" className="rounded border px-2 py-1 text-xs" onClick={() => applyDescriptionCommand("bold")}>Bold</button>
              <button type="button" className="rounded border px-2 py-1 text-xs" onClick={() => applyDescriptionCommand("italic")}>Italic</button>
              <button type="button" className="rounded border px-2 py-1 text-xs" onClick={() => applyDescriptionCommand("insertUnorderedList")}>Bullet</button>
              <button type="button" className="rounded border px-2 py-1 text-xs" onClick={() => applyDescriptionCommand("insertOrderedList")}>Numbered</button>
              <button type="button" className="rounded border px-2 py-1 text-xs" onClick={() => applyDescriptionCommand("createLink")}>Link</button>
            </div>
            <div
              ref={descriptionEditorRef}
              contentEditable
              suppressContentEditableWarning
              className="min-h-36 w-full px-3 py-2 text-sm outline-none [&_a]:text-blue-600 [&_a]:underline [&_img]:my-3 [&_img]:max-w-full [&_img]:rounded [&_ol]:ml-5 [&_ol]:list-decimal [&_p]:my-1 [&_strong]:font-semibold [&_ul]:ml-5 [&_ul]:list-disc"
              onInput={(e) =>
                setDescription(
                  sanitizeRichHtml((e.currentTarget as HTMLDivElement).innerHTML),
                )
              }
            />
          </div>
        </label>
      </section>

      <section className="rounded-lg border border-slate-200 p-3 space-y-3">
        <h4 className="text-sm font-semibold text-slate-700">Event image</h4>
        <div className="text-sm">
          <span className="block mb-1 font-medium">
            Event image (square crop, JPG/PNG/WEBP, max 2MB)
          </span>
          <div ref={uploadTriggerRef} className="relative z-0 overflow-hidden">
            <ImgCrop rotationSlider cropShape="rect" aspect={1} showGrid>
              {isMobile ? (
                <Upload {...uploadProps} showUploadList={false}>
                  <div className="w-full rounded-lg border border-dashed border-slate-300 bg-white px-3 py-4 text-center text-sm">
                    Tap to upload event image
                  </div>
                </Upload>
              ) : (
                <Upload.Dragger
                  {...uploadProps}
                  multiple={false}
                  showUploadList={false}
                  className="!w-full !min-h-[132px] !p-3"
                >
                  <p className="text-sm font-medium">Drag & drop event image here, or click to upload</p>
                  <p className="text-xs text-slate-500">Only 1 image. Crop is required before upload.</p>
                </Upload.Dragger>
              )}
            </ImgCrop>
          </div>
          {importedLumaImageURL && (
            <div className="mt-2 rounded-lg border border-amber-300 bg-amber-50 p-2">
              <p className="text-xs text-amber-700">
                Imported Luma image detected. Upload and crop one image before saving.
              </p>
              <img src={importedLumaImageURL} alt="Imported Luma event" className="mt-2 h-20 w-20 rounded border object-cover" />
            </div>
          )}
          {eventImageUrl && (
            <img src={eventImageUrl} alt="Event preview" className="mt-2 h-28 w-28 rounded-lg border object-cover" />
          )}
          {eventFormErrors.eventImageUrl && (
            <p className="mt-1 text-xs text-rose-600">{eventFormErrors.eventImageUrl}</p>
          )}
        </div>
      </section>

      {detailMode === "edit" && selectedEvent && (
        <div className="rounded-lg border bg-slate-50 p-3">
          <p className="text-xs text-slate-500 mb-1">Checkout link</p>
          <div className="font-mono text-xs break-all">{fullCheckoutLink}</div>
          <div className="flex gap-2 mt-2">
            <button onClick={() => copyText(fullCheckoutLink)} className="rounded-lg border px-2.5 py-1 text-xs">Copy Link</button>
            <a href={`/checkout/${selectedEvent.slug}`} target="_blank" rel="noreferrer" className="rounded-lg border px-2.5 py-1 text-xs">Open Checkout</a>
          </div>
        </div>
      )}
    </div>
  );


  const renderCheckoutFormSection = () => (
    <div className="space-y-3">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <label className="text-sm md:col-span-2">
          <span className="block mb-1 font-medium">
            Merchant wallet address <span className="text-rose-600">*</span>
          </span>
          <input
            className={`border rounded-lg px-3 py-2 w-full font-mono text-xs ${
              eventFormErrors.merchantWallet ? "border-rose-500" : ""
            }`}
            value={merchantWallet}
            onChange={(e) => {
              setMerchantWallet(e.target.value);
              clearEventFormError("merchantWallet");
            }}
            required
          />
          {eventFormErrors.merchantWallet && (
            <p className="mt-1 text-xs text-rose-600">
              {eventFormErrors.merchantWallet}
            </p>
          )}
        </label>
        <label className="text-sm">
          <span className="block mb-1 font-medium">
            USDC amount <span className="text-rose-600">*</span>
          </span>
          <input
            className={`border rounded-lg px-3 py-2 w-full ${
              eventFormErrors.amountUsdc ? "border-rose-500" : ""
            }`}
            type="number"
            min="1"
            step="1"
            value={amountUsdc}
            onChange={(e) => {
              setAmountUsdc(e.target.value);
              clearEventFormError("amountUsdc");
            }}
            required
          />
          {eventFormErrors.amountUsdc && (
            <p className="mt-1 text-xs text-rose-600">
              {eventFormErrors.amountUsdc}
            </p>
          )}
        </label>
      </div>

      <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 space-y-3">
        <p className="text-sm font-medium">Participant Info Form Fields</p>
        <div className="flex flex-wrap items-end gap-2">
          <label className="text-sm">
            <span className="mb-1 block">Search field</span>
            <input
              className="border rounded-lg px-3 py-2"
              placeholder="Search field..."
              value={checkoutFormSearchTerm}
              onChange={(e) => setCheckoutFormSearchTerm(e.target.value)}
            />
          </label>
          <button
            onClick={addParticipantField}
            className="rounded-lg bg-slate-900 text-white px-3 py-2 text-sm"
          >
            Add field
          </button>
        </div>

        <div className="text-xs text-slate-500">
          Showing{" "}
          {filteredCheckoutFields.length === 0
            ? 0
            : (checkoutFormPage - 1) * 10 + 1}
          -{Math.min(checkoutFormPage * 10, filteredCheckoutFields.length)} of{" "}
          {filteredCheckoutFields.length} fields
        </div>
        <div className="rounded-lg border bg-white p-2 max-h-72 overflow-auto">
          {pagedCheckoutFields.length === 0 ? (
            <p className="text-sm text-slate-500">No fields found.</p>
          ) : (
            <div className="space-y-2">
              {pagedCheckoutFields.map((field) => {
                const absoluteIndex = participantFields.findIndex(
                  (v) => v === field,
                );
                const fixed =
                  field.field_name === "name" || field.field_name === "email";
                return (
                  <div
                    key={`field-${absoluteIndex}`}
                    className="grid grid-cols-1 md:grid-cols-[1fr_88px_76px] gap-2 border rounded-lg bg-white border-slate-200 p-2"
                  >
                    <input
                      className="border rounded px-2 py-1.5 text-xs"
                      value={field.field_name}
                      disabled={fixed}
                      onChange={(e) =>
                        setParticipantFields((prev) =>
                          prev.map((v, i) =>
                            i === absoluteIndex
                              ? {
                                  ...v,
                                  field_name: e.target.value,
                                }
                              : v,
                          ),
                        )
                      }
                    />
                    <label className="inline-flex items-center gap-1 rounded border px-2 py-1 text-xs">
                      <input
                        type="checkbox"
                        disabled={fixed}
                        checked={field.required}
                        onChange={(e) =>
                          setParticipantFields((prev) =>
                            prev.map((v, i) =>
                              i === absoluteIndex
                                ? { ...v, required: e.target.checked }
                                : v,
                            ),
                          )
                        }
                      />
                      Required
                    </label>
                    <button
                      onClick={() => {
                        if (fixed) return;
                        setParticipantFields((prev) =>
                          prev.filter((_, i) => i !== absoluteIndex),
                        );
                      }}
                      disabled={fixed}
                      className="rounded border px-2 py-1 text-xs disabled:opacity-40"
                    >
                      Remove
                    </button>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
      <div
        className={`border-t border-slate-200 -mx-3 px-3 py-2 flex items-center gap-2 ${isMobile ? "sticky bottom-0 bg-white/95 backdrop-blur" : "bg-white"}`}
      >
        <button
          disabled={checkoutFormPage <= 1}
          onClick={() => setCheckoutFormPage((p) => Math.max(1, p - 1))}
          className="rounded-lg border px-2.5 py-1 text-xs disabled:opacity-50"
        >
          Prev
        </button>
        <span className="text-xs text-slate-600">
          Page {checkoutFormPage} / {checkoutFormTotalPages}
        </span>
        <button
          disabled={checkoutFormPage >= checkoutFormTotalPages}
          onClick={() =>
            setCheckoutFormPage((p) => Math.min(checkoutFormTotalPages, p + 1))
          }
          className="rounded-lg border px-2.5 py-1 text-xs disabled:opacity-50"
        >
          Next
        </button>
      </div>
    </div>
  );

  const renderDepositList = () => (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end gap-2">
        <label className="text-sm">
          <span className="mb-1 block">Search Deposit</span>
          <input
            className="border rounded-lg px-3 py-2"
            placeholder="Wallet, reference, status..."
            value={depositSearchTerm}
            onChange={(e) => setDepositSearchTerm(e.target.value)}
          />
        </label>
        <button
          onClick={() => refreshCheckouts()}
          disabled={detailMode === "create"}
          className="rounded-lg border px-3 py-2 text-sm disabled:opacity-50"
        >
          Refresh
        </button>
      </div>
      {detailMode === "create" ? (
        <p className="text-xs text-slate-500">
          Save event first to view participant deposits.
        </p>
      ) : loadingCheckouts ? (
        <p className="text-sm text-slate-500">Loading deposits...</p>
      ) : pagedDeposits.length === 0 ? (
        <p className="text-sm text-slate-500">No deposits found.</p>
      ) : (
        <>
          <div className="text-xs text-slate-500">
            Showing{" "}
            {filteredDeposits.length === 0 ? 0 : (depositPage - 1) * 10 + 1}-
            {Math.min(depositPage * 10, filteredDeposits.length)} of{" "}
            {filteredDeposits.length} deposits
          </div>
          <div className="rounded-lg border overflow-auto max-h-80">
            <table className="min-w-full text-xs">
              <thead className="bg-slate-50">
                <tr className="text-left">
                  {participantFields
                    .filter((f) => f.required)
                    .map((field) => (
                      <th
                        key={`required-${field.field_name}`}
                        className="px-2 py-2"
                      >
                        {field.field_name}
                      </th>
                    ))}
                  <th className="px-2 py-2">Additional info</th>
                  <th className="px-2 py-2">Wallet</th>
                  <th className="px-2 py-2">Status</th>
                  <th className="px-2 py-2">Transaction</th>
                </tr>
              </thead>
              <tbody>
                {pagedDeposits.map((row) => (
                  <tr key={row.id} className="border-t align-top">
                    {participantFields
                      .filter((f) => f.required)
                      .map((field) => (
                        <td
                          key={`required-${row.id}-${field.field_name}`}
                          className="px-2 py-2"
                        >
                          {row.participantData?.[field.field_name] ?? "-"}
                        </td>
                      ))}
                    <td className="px-2 py-2 max-w-[220px]">
                      {(() => {
                        const required = new Set(
                          participantFields
                            .filter((f) => f.required)
                            .map((f) => f.field_name),
                        );
                        const extras = Object.entries(row.participantData || {})
                          .filter(
                            ([k, v]) =>
                              !required.has(k) && String(v ?? "").trim() !== "",
                          )
                          .map(([k, v]) => `${k}: ${String(v)}`);
                        if (extras.length === 0) return "-";
                        return (
                          <div className="space-y-1">
                            {extras.map((line) => (
                              <div
                                key={`${row.id}-${line}`}
                                className="break-words"
                              >
                                {line}
                              </div>
                            ))}
                          </div>
                        );
                      })()}
                    </td>
                    <td className="px-2 py-2 font-mono max-w-[140px] break-all">
                      {row.walletAddress}
                    </td>
                    <td className="px-2 py-2">
                      <div>{row.status}</div>
                    </td>
                    <td className="px-2 py-2">
                      {row.signature && (
                        <a
                          href={
                            (process.env.NEXT_PUBLIC_USDC_MINT ||
                              "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU") !==
                            "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"
                              ? `https://solscan.io/tx/${row.signature}`
                              : `https://solscan.io/tx/${row.signature}?cluster=devnet`
                          }
                          target="_blank"
                          rel="noreferrer"
                          className="underline text-blue-600 block mt-1"
                        >
                          Tx
                        </a>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div
            className={`border-t border-slate-200 -mx-3 px-3 py-2 flex items-center gap-2 ${isMobile ? "sticky bottom-0 bg-white/95 backdrop-blur" : "bg-white"}`}
          >
            <button
              disabled={depositPage <= 1}
              onClick={() => setDepositPage((p) => Math.max(1, p - 1))}
              className="rounded-lg border px-2.5 py-1 text-xs disabled:opacity-50"
            >
              Prev
            </button>
            <span className="text-xs text-slate-600">
              Page {depositPage} / {depositTotalPages}
            </span>
            <button
              disabled={depositPage >= depositTotalPages}
              onClick={() =>
                setDepositPage((p) => Math.min(depositTotalPages, p + 1))
              }
              className="rounded-lg border px-2.5 py-1 text-xs disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  );

  if (authLoading) {
    return (
      <main className="min-h-screen bg-slate-50 text-slate-900">
        <div className="mx-auto max-w-lg px-4 py-14">
          <section className="rounded-2xl bg-white border border-slate-200 shadow-sm p-6">
            <p className="text-sm text-slate-600">Checking session...</p>
          </section>
        </div>
      </main>
    );
  }

  if (!currentUser) {
    return (
      <main className="min-h-screen bg-slate-50 text-slate-900">
        <div className="mx-auto max-w-lg px-4 py-14">
          <section className="rounded-2xl bg-white border border-slate-200 shadow-sm p-6 space-y-4">
            <div className="flex items-center gap-3">
              <Image
                src="/payknot_nontext.svg"
                alt="Payknot"
                width={36}
                height={36}
                className="h-9 w-9"
              />
              <div>
                <h1 className="text-2xl font-bold">Login / Register</h1>
                <p className="text-xs text-slate-500">Payknot</p>
              </div>
            </div>
            <p className="text-sm text-slate-600">
              Login required to access event checkout management.
            </p>

            <div className="flex gap-2">
              <button
                onClick={() => setAuthMode("login")}
                className={`px-3 py-1.5 rounded-lg border ${authMode === "login" ? "bg-slate-900 text-white" : "bg-white"}`}
              >
                Login
              </button>
              <button
                onClick={() => setAuthMode("register")}
                className={`px-3 py-1.5 rounded-lg border ${authMode === "register" ? "bg-slate-900 text-white" : "bg-white"}`}
              >
                Register
              </button>
            </div>

            <form onSubmit={onAuthSubmit} className="space-y-3">
              {authMode === "register" && (
                <label className="text-sm block">
                  <span className="block mb-1">Name</span>
                  <input
                    className="border rounded-lg px-3 py-2 w-full"
                    value={authName}
                    onChange={(e) => setAuthName(e.target.value)}
                  />
                </label>
              )}
              <label className="text-sm block">
                <span className="block mb-1">Email</span>
                <input
                  className="border rounded-lg px-3 py-2 w-full"
                  type="email"
                  value={authEmail}
                  onChange={(e) => setAuthEmail(e.target.value)}
                />
              </label>
              <label className="text-sm block">
                <span className="block mb-1">Password</span>
                <input
                  className="border rounded-lg px-3 py-2 w-full"
                  type="password"
                  value={authPassword}
                  onChange={(e) => setAuthPassword(e.target.value)}
                />
              </label>
              <button
                disabled={authSubmitting}
                className="w-full rounded-lg bg-slate-900 text-white py-2.5 font-semibold disabled:opacity-60"
              >
                {authSubmitting
                  ? "Please wait..."
                  : authMode === "register"
                    ? "Create Account"
                    : "Login"}
              </button>
            </form>

            <div className="pt-2 border-t">
              <p className="text-sm mb-2">Or continue with Google</p>
              {process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ? (
                <div id="google-login-button" />
              ) : (
                <p className="text-xs text-amber-700">
                  Set NEXT_PUBLIC_GOOGLE_CLIENT_ID to enable Google OAuth.
                </p>
              )}
            </div>

            {error && <p className="text-sm text-red-600">{error}</p>}
          </section>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-slate-50 text-slate-900">
      <div className="mx-auto max-w-6xl px-3 md:px-4 py-4 md:py-10 space-y-4 md:space-y-6">
        <section
          ref={pageHeaderRef}
          className={`sticky top-0 z-30 rounded-2xl bg-white/95 backdrop-blur border border-slate-200 shadow-sm p-3 md:p-6 transition-transform duration-300 ${!isMobile && sessionsHeaderPinned ? "-translate-y-[105%]" : "translate-y-0"}`}
        >
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-3">
              <Image
                src="/payknot_nontext.svg"
                alt="Payknot"
                width={42}
                height={42}
                className="h-9 w-9 md:h-10 md:w-10"
              />
              <div>
                <h1 className="text-xl md:text-3xl font-bold tracking-tight">
                  Payknot
                </h1>
                <p className="mt-1 text-slate-600 text-xs md:text-sm">
                  Signed in as {currentUser.name} ({currentUser.email})
                </p>
              </div>
            </div>
            <button
              onClick={logout}
              className="rounded-lg border px-3 py-1.5 md:py-2 text-xs md:text-sm"
            >
              Logout
            </button>
          </div>

          <div className="mt-3 md:hidden space-y-2">
            <div className="grid grid-cols-2 gap-2">
              <button
                onClick={() => {
                  if (currentUser) void loadEvents();
                }}
                className="rounded-lg border px-2 py-1.5 text-xs"
                disabled={loadingEvents}
              >
                {loadingEvents ? "Loading..." : "Refresh"}
              </button>
              <button
                onClick={openCreateDetail}
                className="rounded-lg border px-2 py-1.5 text-xs"
              >
                Create Event
              </button>
            </div>
            <input
              className="border rounded-lg px-2 py-1.5 w-full text-xs"
              placeholder="Search title, organizer, description, wallet, location"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
            <div className="grid grid-cols-2 gap-2">
              <input
                className="border rounded-lg px-2 py-1.5 w-full text-xs"
                type="date"
                value={eventDateFrom}
                onChange={(e) => setEventDateFrom(e.target.value)}
              />
              <input
                className="border rounded-lg px-2 py-1.5 w-full text-xs"
                type="date"
                value={eventDateTo}
                onChange={(e) => setEventDateTo(e.target.value)}
              />
            </div>
            <div className="grid grid-cols-3 gap-2">
              <select
                className="border rounded-lg px-2 py-1.5 w-full text-xs"
                value={sortField}
                onChange={(e) => setSortField(e.target.value as EventSortField)}
              >
                <option value="title">Name</option>
                <option value="organizerName">Organizer</option>
                <option value="eventDate">Date</option>
                <option value="amountUsdc">USDC</option>
              </select>
              <select
                className="border rounded-lg px-2 py-1.5 w-full text-xs"
                value={sortDirection}
                onChange={(e) =>
                  setSortDirection(e.target.value as "asc" | "desc")
                }
              >
                <option value="desc">Desc</option>
                <option value="asc">Asc</option>
              </select>
              <button
                onClick={() => {
                  setSearchQuery("");
                  setEventDateFrom("");
                  setEventDateTo("");
                  setSortField("eventDate");
                  setSortDirection("desc");
                }}
                className="rounded-lg border px-2 py-1.5 text-xs"
              >
                Reset
              </button>
            </div>
          </div>
        </section>

        <section className="rounded-2xl bg-white border border-slate-200 shadow-sm p-3 md:p-6 space-y-3">
          <div
            ref={sessionsHeaderRef}
            className="hidden md:block sticky z-20 bg-white/95 backdrop-blur rounded-lg border border-slate-200 p-3 flex-col gap-3  transition-all duration-300"
            style={{ top: sessionsHeaderPinned ? 0 : desktopHeaderHeight + 8 }}
          >
            <div className="md:flex md:flex-row md:items-center md:justify-between mb-5">
              <h2 className="text-xl font-semibold">Your Event Sessions</h2>
              <div className="flex flex-wrap gap-2">
                <button
                  onClick={() => {
                    if (currentUser) void loadEvents();
                  }}
                  className="rounded-lg border px-3 py-1.5 text-sm"
                  disabled={loadingEvents}
                >
                  {loadingEvents ? "Loading..." : "Refresh"}
                </button>
                <button
                  onClick={openCreateDetail}
                  className="rounded-lg border px-3 py-1.5 text-sm"
                >
                  Create Event
                </button>
              </div>
            </div>
            <div className="hidden md:grid grid-cols-1 gap-3 md:grid-cols-3 lg:grid-cols-6">
              <label className="text-sm">
                <span className="mb-1 block">Search</span>
                <input
                  className="border rounded-lg px-3 py-2 w-full"
                  placeholder="Title, organizer, description, wallet, location"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </label>
              <label className="text-sm">
                <span className="mb-1 block">Event date from</span>
                <input
                  className="border rounded-lg px-3 py-2 w-full"
                  type="date"
                  value={eventDateFrom}
                  onChange={(e) => setEventDateFrom(e.target.value)}
                />
              </label>
              <label className="text-sm">
                <span className="mb-1 block">Event date to</span>
                <input
                  className="border rounded-lg px-3 py-2 w-full"
                  type="date"
                  value={eventDateTo}
                  onChange={(e) => setEventDateTo(e.target.value)}
                />
              </label>
              <label className="text-sm">
                <span className="mb-1 block">Sort by</span>
                <select
                  className="border rounded-lg px-3 py-2 w-full"
                  value={sortField}
                  onChange={(e) =>
                    setSortField(e.target.value as EventSortField)
                  }
                >
                  <option value="title">Event name</option>
                  <option value="organizerName">Organizer name</option>
                  <option value="eventDate">Event date & time</option>
                  <option value="amountUsdc">USDC amount</option>
                </select>
              </label>
              <label className="text-sm">
                <span className="mb-1 block">Order</span>
                <select
                  className="border rounded-lg px-3 py-2 w-full"
                  value={sortDirection}
                  onChange={(e) =>
                    setSortDirection(e.target.value as "asc" | "desc")
                  }
                >
                  <option value="desc">Descending</option>
                  <option value="asc">Ascending</option>
                </select>
              </label>
              <div className="flex items-end">
                <button
                  onClick={() => {
                    setSearchQuery("");
                    setEventDateFrom("");
                    setEventDateTo("");
                    setSortField("eventDate");
                    setSortDirection("desc");
                  }}
                  className="rounded-lg border px-3 py-2 text-sm w-full"
                >
                  Reset Filters
                </button>
              </div>
            </div>
          </div>

          {events.length === 0 ? (
            <p className="text-sm text-slate-500">
              No event created yet for this account.
            </p>
          ) : filteredAndSortedEvents.length === 0 ? (
            <p className="text-sm text-slate-500">
              No event matches your current search/filter.
            </p>
          ) : (
            <>
              <div className="text-xs text-slate-500">
                Showing {(eventPage - 1) * EVENT_PAGE_SIZE + 1}-
                {Math.min(
                  eventPage * EVENT_PAGE_SIZE,
                  filteredAndSortedEvents.length,
                )}{" "}
                of {filteredAndSortedEvents.length} events
              </div>
              <div className="grid grid-cols-1 gap-2 md:gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {paginatedEvents.map((ev) => (
                  <button
                    key={ev.eventId}
                    onClick={() => openEditDetail(ev.eventId)}
                    className={`text-left border rounded-xl p-2 md:p-3 transition ${selectedEventID === ev.eventId ? "border-slate-900 bg-slate-100" : "border-slate-200 bg-white"}`}
                  >
                    <div className="flex gap-2 md:block">
                      {ev.eventImageUrl && (
                        <img
                          src={ev.eventImageUrl}
                          alt={ev.title}
                          className="h-12 w-12 rounded-md object-cover flex-shrink-0 md:w-full md:h-auto md:aspect-square md:mb-2"
                        />
                      )}
                      <div className="min-w-0 flex-1">
                        <p className="font-semibold line-clamp-1 text-xs md:text-base">
                          {ev.title}
                        </p>
                        <div className="mt-1 space-y-0.5 text-[11px] md:text-xs text-slate-600">
                          <p className="line-clamp-1">
                            Org: {ev.organizerName || "-"}
                          </p>
                          <p className="line-clamp-1">
                            {ev.eventDate
                              ? new Date(ev.eventDate).toLocaleDateString()
                              : "-"}
                          </p>
                          <p className="line-clamp-1">{ev.amountUsdc} USDC</p>
                          <p className="hidden md:block line-clamp-1 font-mono text-[11px]">
                            {ev.merchantWallet}
                          </p>
                        </div>
                      </div>
                    </div>
                  </button>
                ))}
              </div>

              <div className="sticky bottom-0 bg-white/95 backdrop-blur border-t border-slate-200 -mx-3 md:mx-0 md:border-0 md:bg-transparent md:static px-3 md:px-0 py-2 flex flex-wrap items-center gap-2 pt-2">
                <button
                  disabled={eventPage <= 1}
                  onClick={() => setEventPage((p) => Math.max(1, p - 1))}
                  className="rounded-lg border px-2.5 py-1 text-xs md:text-sm disabled:opacity-50"
                >
                  Prev
                </button>
                <span className="text-xs md:text-sm text-slate-600">
                  Page {eventPage} / {eventTotalPages}
                </span>
                <button
                  disabled={eventPage >= eventTotalPages}
                  onClick={() =>
                    setEventPage((p) => Math.min(eventTotalPages, p + 1))
                  }
                  className="rounded-lg border px-2.5 py-1 text-xs md:text-sm disabled:opacity-50"
                >
                  Next
                </button>
              </div>
            </>
          )}
        </section>
        {detailRender && (
          <div className="fixed inset-0 z-50">
            <div
              className={`absolute inset-0 bg-slate-900/30 ${detailClosing ? "animate-overlay-fade-out" : "animate-overlay-fade-in"}`}
              onClick={closeDetail}
            />

            {isMobile ? (
              <div
                className={`absolute inset-0 bg-white flex flex-col ${detailClosing ? "animate-panel-down-out" : "animate-panel-up-in"}`}
              >
                <div className="border-b border-slate-200 px-4 py-3 flex items-start justify-between gap-3">
                  <div>
                    <p className="text-xs uppercase tracking-wide text-slate-500">
                      {detailMode === "create" ? "Create Event" : "Edit Event"}
                    </p>
                    <h3 className="text-lg font-semibold">
                      {title || selectedEvent?.title || "New Event"}
                    </h3>
                  </div>
                  <button
                    onClick={closeDetail}
                    className="rounded-lg border px-3 py-1.5 text-sm"
                  >
                    Close
                  </button>
                </div>

                <div className="px-4 py-2 border-b border-slate-200 flex gap-2">
                  {(["info", "checkoutForm", "deposits"] as DetailTab[]).map(
                    (tab) => (
                      <button
                        key={tab}
                        onClick={() => setDetailTab(tab)}
                        className={`flex-1 rounded-lg px-3 py-2 text-sm capitalize border ${detailTab === tab ? "bg-slate-900 text-white border-slate-900" : "bg-white border-slate-300"}`}
                      >
                        {tab === "info"
                          ? "General Info"
                          : tab === "checkoutForm"
                            ? "Checkout Form"
                            : "Deposits"}
                      </button>
                    ),
                  )}
                </div>

                <div className="flex-1 overflow-y-auto px-4 py-4 pb-28 space-y-4">
                  {detailTab === "info" && renderGeneralInfoForm()}

                  {detailTab === "checkoutForm" && (
                    <section className="rounded-lg border border-slate-200">
                      <button
                        className="w-full px-3 py-2 text-left text-sm font-medium border-b bg-slate-50"
                        onClick={() => setCheckoutFormAccordionOpen((v) => !v)}
                      >
                        Checkout Form {checkoutFormAccordionOpen ? "▾" : "▸"}
                      </button>
                      {checkoutFormAccordionOpen && (
                        <div className="p-3">{renderCheckoutFormSection()}</div>
                      )}
                    </section>
                  )}

                  {detailTab === "deposits" && (
                    <section className="rounded-lg border border-slate-200">
                      <button
                        className="w-full px-3 py-2 text-left text-sm font-medium border-b bg-slate-50"
                        onClick={() => setDepositAccordionOpen((v) => !v)}
                      >
                        Participant Deposits {depositAccordionOpen ? "▾" : "▸"}
                      </button>
                      {depositAccordionOpen && (
                        <div className="p-3">{renderDepositList()}</div>
                      )}
                    </section>
                  )}
                </div>

                <div className="fixed bottom-0 left-0 right-0 border-t border-slate-200 bg-white px-4 py-3">
                  {detailMode === "edit" && hasPaidDeposits && (
                    <p className="mb-2 text-xs text-amber-700">
                      This event already has successful deposits. Edit/Delete is
                      disabled.
                    </p>
                  )}
                  <div className="flex gap-2">
                    {detailMode === "edit" && (
                      <button
                        onClick={deleteEvent}
                        disabled={deleting || !selectedEvent || hasPaidDeposits}
                        className="rounded-lg bg-red-600 text-white px-3 py-2 text-sm disabled:opacity-60"
                      >
                        {deleting ? "Deleting..." : "Delete"}
                      </button>
                    )}
                    <button
                      onClick={saveEventChanges}
                      disabled={
                        creating ||
                        updating ||
                        (detailMode === "edit" && hasPaidDeposits)
                      }
                      className="flex-1 rounded-lg bg-slate-900 text-white py-2 text-sm font-semibold disabled:opacity-60"
                    >
                      {detailMode === "create"
                        ? creating
                          ? "Creating..."
                          : "Save Event"
                        : updating
                          ? "Saving..."
                          : "Save Changes"}
                    </button>
                  </div>
                </div>
              </div>
            ) : (
              <div
                className={`absolute inset-y-0 right-0 w-[600px] bg-white border-l border-slate-200 shadow-xl flex flex-col ${detailClosing ? "animate-panel-right-out" : "animate-panel-right-in"}`}
              >
                <div className="border-b border-slate-200 px-5 py-4 flex items-start justify-between gap-3">
                  <div>
                    <p className="text-xs uppercase tracking-wide text-slate-500">
                      {detailMode === "create"
                        ? "Create Event"
                        : "Event Detail"}
                    </p>
                    <h3 className="text-lg font-semibold">
                      {title || selectedEvent?.title || "New Event"}
                    </h3>
                  </div>
                  <button
                    onClick={closeDetail}
                    className="rounded-lg border px-3 py-1.5 text-sm"
                  >
                    Close
                  </button>
                </div>

                <div className="flex-1 overflow-y-auto p-5 pb-28 space-y-6">
                  <section className="space-y-3">
                    <h4 className="text-sm font-semibold text-slate-700">
                      General Info
                    </h4>
                    {renderGeneralInfoForm()}
                  </section>

                  <section className="space-y-3">
                    <div className="flex items-center justify-between">
                      <h4 className="text-sm font-semibold text-slate-700">
                        Checkout Form
                      </h4>
                      <button
                        onClick={() => setShowCheckoutFormSection((v) => !v)}
                        className="text-xs rounded-lg border px-2 py-1"
                      >
                        {showCheckoutFormSection ? "Hide" : "View All"}
                      </button>
                    </div>
                    {showCheckoutFormSection && renderCheckoutFormSection()}
                  </section>

                  <section className="space-y-3">
                    <div className="flex items-center justify-between">
                      <h4 className="text-sm font-semibold text-slate-700">
                        Participant Deposits
                      </h4>
                      <button
                        onClick={() => setShowDepositsSection((v) => !v)}
                        className="text-xs rounded-lg border px-2 py-1"
                      >
                        {showDepositsSection ? "Hide" : "View All"}
                      </button>
                    </div>
                    {showDepositsSection && renderDepositList()}
                  </section>
                </div>

                <div className="absolute bottom-0 left-0 right-0 border-t border-slate-200 bg-white px-5 py-3">
                  {detailMode === "edit" && hasPaidDeposits && (
                    <p className="mb-2 text-xs text-amber-700">
                      This event already has successful deposits. Edit/Delete is
                      disabled.
                    </p>
                  )}
                  <div className="flex gap-2">
                    {detailMode === "edit" && (
                      <button
                        onClick={deleteEvent}
                        disabled={deleting || !selectedEvent || hasPaidDeposits}
                        className="rounded-lg bg-red-600 text-white px-3 py-2 text-sm disabled:opacity-60"
                      >
                        {deleting ? "Deleting..." : "Delete"}
                      </button>
                    )}
                    <button
                      onClick={saveEventChanges}
                      disabled={
                        creating ||
                        updating ||
                        (detailMode === "edit" && hasPaidDeposits)
                      }
                      className="flex-1 rounded-lg bg-slate-900 text-white py-2 text-sm font-semibold disabled:opacity-60"
                    >
                      {detailMode === "create"
                        ? creating
                          ? "Creating..."
                          : "Save Event"
                        : updating
                          ? "Saving..."
                          : "Save Changes"}
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
        {error && <p className="text-sm text-red-600">{error}</p>}
      </div>
    </main>
  );
}
