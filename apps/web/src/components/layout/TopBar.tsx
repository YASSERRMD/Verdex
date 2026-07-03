'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { UserCircleIcon, LogOutIcon, ChevronDownIcon } from 'lucide-react';
import { clearSession } from '@/lib/auth';
import type { AuthUser } from '@/lib/auth';
import { NotificationBell } from './NotificationBell';

interface TopBarProps {
  user?: AuthUser | null;
}

export function TopBar({ user }: TopBarProps) {
  const router = useRouter();
  const [menuOpen, setMenuOpen] = useState(false);

  const handleLogout = () => {
    clearSession();
    router.push('/login');
  };

  return (
    <header className="flex h-16 items-center justify-between border-b border-neutral-200 bg-white px-6 dark:border-neutral-700 dark:bg-neutral-800">
      {/* Left slot — page breadcrumb placeholder */}
      <div />

      {/* Right slot */}
      <div className="flex items-center gap-3">
        <NotificationBell />

        {/* User menu */}
        <div className="relative">
          <button
            type="button"
            onClick={() => setMenuOpen((v) => !v)}
            className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-neutral-700 hover:bg-neutral-100 dark:text-neutral-300 dark:hover:bg-neutral-700"
            aria-haspopup="menu"
            aria-expanded={menuOpen}
          >
            <UserCircleIcon className="h-6 w-6 text-primary-DEFAULT" aria-hidden="true" />
            <span className="hidden sm:inline">{user?.name ?? 'Account'}</span>
            <ChevronDownIcon className="h-4 w-4 text-neutral-400" aria-hidden="true" />
          </button>

          {menuOpen && (
            <div
              role="menu"
              className="absolute right-0 top-12 z-50 w-52 rounded-xl border border-neutral-200 bg-white py-2 shadow-lg dark:border-neutral-700 dark:bg-neutral-800"
            >
              {user && (
                <div className="border-b border-neutral-100 px-4 py-2 dark:border-neutral-700">
                  <p className="truncate text-sm font-medium text-neutral-800 dark:text-white">
                    {user.name}
                  </p>
                  <p className="truncate text-xs text-neutral-500">{user.email}</p>
                </div>
              )}
              <button
                role="menuitem"
                type="button"
                onClick={handleLogout}
                className="flex w-full items-center gap-2 px-4 py-2.5 text-sm text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
              >
                <LogOutIcon className="h-4 w-4" aria-hidden="true" />
                Sign out
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
