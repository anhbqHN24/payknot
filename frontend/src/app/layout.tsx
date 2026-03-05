import type { Metadata } from 'next';
import { WalletContextProvider } from '@/components/WalletProvider';
import 'antd/dist/reset.css';
import './globals.css';

export const metadata: Metadata = {
  title: 'Event Deposit Checkout',
  description: 'Create invite-code checkout links and collect USDC deposits on Solana',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <WalletContextProvider>{children}</WalletContextProvider>
      </body>
    </html>
  );
}
