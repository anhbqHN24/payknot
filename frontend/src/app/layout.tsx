import type { Metadata } from "next";
import { WalletContextProvider } from "@/components/WalletProvider";
import ThemeToggle from "@/components/ThemeToggle";
import "antd/dist/reset.css";
import "./globals.css";

export const metadata: Metadata = {
  title: "PayKnot - Solana Event Ticketing",
  description:
    "Create event checkout links, collect participant details, and receive USDC deposits on Solana",
  icons: {
    icon: "/payknot_icon.svg",
    shortcut: "/payknot_icon.svg",
    apple: "/payknot_icon.svg",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const themeBootScript = `(() => {
    try {
      const saved = localStorage.getItem('theme');
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      const mode = saved || (prefersDark ? 'dark' : 'light');
      if (mode === 'dark') document.documentElement.classList.add('dark');
    } catch (_) {}
  })();`;

  return (
    <html lang="en" suppressHydrationWarning>
      <body className="app-bg">
        <script dangerouslySetInnerHTML={{ __html: themeBootScript }} />
        <div className="magic-bg" />
        <WalletContextProvider>{children}</WalletContextProvider>
        <ThemeToggle />
      </body>
    </html>
  );
}
