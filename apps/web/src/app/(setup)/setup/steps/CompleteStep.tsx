'use client';

import Link from 'next/link';
import { CheckCircleIcon } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Disclaimer } from '@/components/Disclaimer';

export function CompleteStep() {
  return (
    <div className="space-y-6 text-center">
      <div className="flex flex-col items-center gap-3">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-green-100">
          <CheckCircleIcon className="h-9 w-9 text-green-600" aria-hidden="true" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-800">Setup Complete</h2>
        <p className="text-sm text-neutral-500">
          Your Verdex workspace has been configured. You can now begin creating cases and
          generating draft judicial analyses.
        </p>
      </div>

      <Disclaimer />

      <div className="flex flex-col items-center gap-3">
        <Link href="/dashboard">
          <Button variant="primary" size="lg">
            Go to Dashboard
          </Button>
        </Link>
        <p className="text-xs text-neutral-400">
          You can update these settings at any time from the Settings page.
        </p>
      </div>
    </div>
  );
}
