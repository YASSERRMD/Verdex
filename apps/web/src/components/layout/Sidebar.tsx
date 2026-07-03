'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import clsx from 'clsx';
import {
  LayoutDashboardIcon,
  FolderOpenIcon,
  SearchIcon,
  GlobeIcon,
  SettingsIcon,
  ShieldIcon,
} from 'lucide-react';
import type { Role } from '@/types';

interface NavItem {
  href: string;
  label: string;
  icon: React.ElementType;
  adminOnly?: boolean;
}

const NAV_ITEMS: NavItem[] = [
  { href: '/dashboard', label: 'Dashboard', icon: LayoutDashboardIcon },
  { href: '/cases', label: 'Cases', icon: FolderOpenIcon },
  { href: '/search', label: 'Search', icon: SearchIcon },
  { href: '/jurisdictions', label: 'Jurisdictions', icon: GlobeIcon },
  { href: '/settings', label: 'Settings', icon: SettingsIcon },
  { href: '/admin', label: 'Admin', icon: ShieldIcon, adminOnly: true },
];

interface SidebarProps {
  roles?: Role[];
}

export function Sidebar({ roles = [] }: SidebarProps) {
  const pathname = usePathname();
  const isAdmin = roles.includes('admin');

  const visibleItems = NAV_ITEMS.filter((item) => !item.adminOnly || isAdmin);

  return (
    <aside className="flex h-full w-64 flex-col border-r border-neutral-200 bg-white dark:border-neutral-700 dark:bg-neutral-800">
      {/* Wordmark */}
      <div className="flex h-16 items-center gap-3 border-b border-neutral-200 px-6 dark:border-neutral-700">
        <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-accent-DEFAULT">
          <span className="text-sm font-bold text-primary-900">V</span>
        </div>
        <span className="text-lg font-bold text-primary-DEFAULT dark:text-white">Verdex</span>
      </div>

      {/* Nav */}
      <nav className="flex-1 overflow-y-auto px-3 py-4" aria-label="Main navigation">
        <ul className="space-y-1">
          {visibleItems.map((item) => {
            const Icon = item.icon;
            const active =
              item.href === '/dashboard'
                ? pathname === '/dashboard'
                : pathname.startsWith(item.href);
            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  className={clsx(
                    'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                    active
                      ? 'bg-primary-50 text-primary-DEFAULT dark:bg-primary-900/40 dark:text-primary-200'
                      : 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900 dark:text-neutral-300 dark:hover:bg-neutral-700 dark:hover:text-white',
                  )}
                  aria-current={active ? 'page' : undefined}
                >
                  <Icon className="h-4 w-4 flex-shrink-0" aria-hidden="true" />
                  {item.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>

      {/* Footer */}
      <div className="border-t border-neutral-200 p-4 dark:border-neutral-700">
        <p className="text-xs text-neutral-400">
          Non-binding analysis only. All outputs require judicial review.
        </p>
      </div>
    </aside>
  );
}
