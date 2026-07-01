import type { Metadata } from 'next';
import LoginForm from './LoginForm';

export const metadata: Metadata = {
  title: 'Sign In',
  description: 'Sign in to the Verdex Judicial Reasoning Platform',
};

export default function LoginPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-br from-primary-900 via-primary-800 to-primary-700 px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-8">
        {/* Logo / Wordmark */}
        <div className="text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-accent-DEFAULT">
            <span className="text-2xl font-bold text-primary-900">V</span>
          </div>
          <h1 className="text-3xl font-bold tracking-tight text-white">Verdex</h1>
          <p className="mt-2 text-sm text-primary-200">Judicial Reasoning Platform</p>
        </div>

        {/* Card */}
        <div className="rounded-2xl bg-white px-8 py-10 shadow-2xl">
          <h2 className="mb-6 text-center text-xl font-semibold text-neutral-800">
            Sign in to your account
          </h2>
          <LoginForm />
        </div>

        {/* Non-binding disclaimer */}
        <p className="text-center text-xs text-primary-200">
          This system produces non-binding draft analyses only. All outputs require
          review and sign-off by a qualified judge.
        </p>
      </div>
    </div>
  );
}
