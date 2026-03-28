"use client";
import { useState } from "react";
import Image from "next/image";

const navTabs = [
  { id: "problem", label: "Problem" },
  { id: "how-it-works", label: "How it works" },
  { id: "fit", label: "Fit" },
  { id: "credibility", label: "Credibility" },
  { id: "faq", label: "FAQ" },
  { href: "/agents", label: "Agents" },
];

export default function Header() {
  const [isOpen, setIsOpen] = useState(false);

  const toggleMenu = () => {
    setIsOpen(!isOpen);
  };

  return (
    <header className="sticky top-2 z-30 rounded-2xl border app-surface shadow-sm">
      <div className="mx-auto w-full max-w-6xl px-4">
        <div className="flex items-center justify-between h-16">
          <div className="flex items-center">
            <Image
              src="/payknot_nontext.svg"
              alt="Payknot"
              width={40}
              height={40}
              className="h-9 w-9"
            />
            <a href="#home" className="text-2xl font-bold text-primary ml-2">
              Payknot
            </a>
          </div>
          <div className="hidden md:flex items-center space-x-1">
            {navTabs.map((tab) => (
              <a
                key={tab.id ?? tab.href}
                href={tab.href ?? `#${tab.id}`}
                className="btn-anim rounded-lg border border-transparent hover:border-[var(--app-border)] px-3 py-1.5 text-sm font-medium"
              >
                {tab.label}
              </a>
            ))}
          </div>
          <div className="md:hidden flex items-center">
            <button
              onClick={toggleMenu}
              className="inline-flex items-center justify-center p-2 rounded-md app-muted focus:outline-none"
              aria-expanded={isOpen}
            >
              <span className="sr-only">Open main menu</span>
              <svg
                className="h-6 w-6"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-hidden="true"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d={isOpen ? "M6 18L18 6M6 6l12 12" : "M4 6h16M4 12h16M4 18h16"}
                />
              </svg>
            </button>
          </div>
        </div>
      </div>
      <div
        className={`md:hidden ${
          isOpen ? "max-h-96" : "max-h-0"
        } transition-max-height duration-300 ease-in-out overflow-hidden`}
        id="mobile-menu"
      >
        <div className="px-2 pt-2 pb-3 space-y-1 sm:px-3">
          {navTabs.map((tab) => (
            <a
              key={tab.id ?? tab.href}
              href={tab.href ?? `#${tab.id}`}
              onClick={toggleMenu}
              className="block rounded-lg px-3 py-2 text-base font-medium app-muted hover:app-fg"
            >
              {tab.label}
            </a>
          ))}
        </div>
      </div>
    </header>
  );
}
