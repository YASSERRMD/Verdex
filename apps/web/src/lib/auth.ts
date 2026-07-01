export interface AuthUser {
  id: string;
  name: string;
  email: string;
  roles: string[];
}

export interface AuthSession {
  token: string;
  user: AuthUser;
}

const SESSION_KEY = 'verdex_session';

/**
 * Retrieve the current auth session from sessionStorage.
 * Returns null when running server-side or when no session exists.
 */
export function getSession(): AuthSession | null {
  if (typeof window === 'undefined') return null;
  try {
    const raw = sessionStorage.getItem(SESSION_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as AuthSession;
  } catch {
    return null;
  }
}

/**
 * Persist an auth session to sessionStorage.
 */
export function setSession(session: AuthSession): void {
  if (typeof window === 'undefined') return;
  sessionStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

/**
 * Remove the auth session from sessionStorage (logout).
 */
export function clearSession(): void {
  if (typeof window === 'undefined') return;
  sessionStorage.removeItem(SESSION_KEY);
}

/**
 * Check whether the current session token appears to be expired.
 * Parses the JWT payload — does not verify the signature server-side.
 */
export function isTokenExpired(token: string): boolean {
  try {
    const [, payload] = token.split('.');
    if (!payload) return true;
    const decoded = JSON.parse(atob(payload)) as { exp?: number };
    if (!decoded.exp) return false;
    return Date.now() >= decoded.exp * 1000;
  } catch {
    return true;
  }
}

/**
 * Returns true if a valid, non-expired session exists.
 */
export function isAuthenticated(): boolean {
  const session = getSession();
  if (!session) return false;
  return !isTokenExpired(session.token);
}
