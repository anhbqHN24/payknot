import type { Metadata } from "next";
import { WalletContextProvider } from "@/components/WalletProvider";
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
  return (
    <html lang="en">
      <body>
        <WalletContextProvider>{children}</WalletContextProvider>
      </body>
    </html>
  );
}
