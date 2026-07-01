import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import './globals.css';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-inter',
  display: 'swap',
});

export const metadata: Metadata = {
  title: {
    default: 'Verdex — Judicial Reasoning Platform',
    template: '%s | Verdex',
  },
  description:
    'Verdex produces non-binding draft judicial analyses to support judges. All outputs require review and sign-off by a qualified judge.',
  keywords: ['judicial', 'legal', 'reasoning', 'AI', 'draft analysis'],
  robots: {
    index: false,
    follow: false,
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={inter.variable} suppressHydrationWarning>
      <body className="min-h-screen bg-neutral-50 font-sans antialiased dark:bg-neutral-900">
        {children}
      </body>
    </html>
  );
}
