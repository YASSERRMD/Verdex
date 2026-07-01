'use client';

import { useState, useCallback, useEffect } from 'react';
import { apiFetch } from '@/lib/api';
import { getSession, setSession, clearSession, isAuthenticated } from '@/lib/auth';
import type { AuthSession } from '@/lib/auth';

interface LoginResponse {
  token: string;
  user: {
    id: string;
    name: string;
    email: string;
    roles: string[];
  };
}

interface UseAuthReturn {
  session: AuthSession | null;
  isAuthenticated: boolean;
  loading: boolean;
  error: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

export function useAuth(): UseAuthReturn {
  const [session, setSessionState] = useState<AuthSession | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Hydrate session from sessionStorage on mount
  useEffect(() => {
    const stored = getSession();
    if (stored) {
      setSessionState(stored);
    }
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiFetch<LoginResponse>('/api/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
        skipAuth: true,
      });

      const newSession: AuthSession = {
        token: data.token,
        user: {
          id: data.user.id,
          name: data.user.name,
          email: data.user.email,
          roles: data.user.roles,
        },
      };
      setSession(newSession);
      setSessionState(newSession);
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : 'Login failed. Please try again.';
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(() => {
    clearSession();
    setSessionState(null);
    setError(null);
  }, []);

  return {
    session,
    isAuthenticated: isAuthenticated(),
    loading,
    error,
    login,
    logout,
  };
}
