import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { LocaleProvider } from '@/lib/i18n/LocaleContext';
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
      {/*
        suppressHydrationWarning above already tolerates a client-only
        attribute change on this element; LocaleProvider (task 3/6)
        reads the verdex_locale cookie on mount and applies dir/lang
        here to match, the same pattern this app already uses for
        every other piece of client state (see e.g. dashboard/page.tsx
        reading the session in a useEffect rather than server-side --
        this app has no server-rendered initial state anywhere yet).
      */}
      <body className="min-h-screen bg-neutral-50 font-sans antialiased dark:bg-neutral-900">
        <LocaleProvider>{children}</LocaleProvider>
      </body>
    </html>
  );
}
