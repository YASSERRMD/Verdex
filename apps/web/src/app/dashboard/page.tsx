'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { FolderPlusIcon, GlobeIcon, SettingsIcon } from 'lucide-react';
import { getSession } from '@/lib/auth';
import { AppShell } from '@/components/layout/AppShell';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Disclaimer } from '@/components/Disclaimer';
import type { Role } from '@/types';

const QUICK_ACTIONS = [
  {
    href: '/cases/new',
    label: 'New Case',
    description: 'Create a new case and generate a draft analysis',
    icon: FolderPlusIcon,
  },
  {
    href: '/jurisdictions',
    label: 'Jurisdictions',
    description: 'View and manage available jurisdictions',
    icon: GlobeIcon,
  },
  {
    href: '/settings',
    label: 'Settings',
    description: 'Configure your workspace and preferences',
    icon: SettingsIcon,
  },
];

export default function DashboardPage() {
  const router = useRouter();
  const session = getSession();

  useEffect(() => {
    if (!session) {
      router.replace('/login');
    }
  }, [session, router]);

  if (!session) return null;

  const firstName = session.user.name.split(' ')[0] ?? session.user.name;
  const roles = (session.user.roles ?? []) as Role[];

  return (
    <AppShell user={session.user} roles={roles}>
      <div className="mx-auto max-w-5xl space-y-8">
        {/* Greeting */}
        <div>
          <h1 className="text-2xl font-bold text-neutral-900 dark:text-white">
            Welcome back, {firstName}
          </h1>
          <p className="mt-1 text-sm text-neutral-500">
            Here is an overview of your Verdex workspace.
          </p>
        </div>

        {/* Disclaimer */}
        <Disclaimer />

        {/* Stats row */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {[
            { label: 'Open Cases', value: '—' },
            { label: 'Pending Review', value: '—' },
            { label: 'Closed This Month', value: '—' },
          ].map((stat) => (
            <Card key={stat.label} padding="md">
              <p className="text-xs font-medium uppercase tracking-wider text-neutral-500">
                {stat.label}
              </p>
              <p className="mt-2 text-3xl font-bold text-primary-DEFAULT">{stat.value}</p>
            </Card>
          ))}
        </div>

        {/* Quick Actions */}
        <div>
          <h2 className="mb-4 text-base font-semibold text-neutral-700 dark:text-neutral-300">
            Quick Actions
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {QUICK_ACTIONS.map((action) => {
              const Icon = action.icon;
              return (
                <Card key={action.href} padding="md">
                  <div className="flex flex-col gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 dark:bg-primary-900/30">
                      <Icon className="h-5 w-5 text-primary-DEFAULT" aria-hidden="true" />
                    </div>
                    <div>
                      <p className="font-semibold text-neutral-800 dark:text-white">
                        {action.label}
                      </p>
                      <p className="mt-0.5 text-xs text-neutral-500">{action.description}</p>
                    </div>
                    <Link href={action.href}>
                      <Button variant="ghost" size="sm" className="mt-auto w-full">
                        Open
                      </Button>
                    </Link>
                  </div>
                </Card>
              );
            })}
          </div>
        </div>
      </div>
    </AppShell>
  );
}
