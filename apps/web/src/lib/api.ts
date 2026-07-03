import { getSession } from './auth';

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly body?: unknown,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export interface ApiFetchOptions extends RequestInit {
  /** If true, do not automatically inject the Authorization header. */
  skipAuth?: boolean;
}

/**
 * Typed fetch wrapper for the Verdex API.
 *
 * @param path - API path, e.g. "/api/v1/auth/login"
 * @param options - Standard fetch options plus `skipAuth`
 * @returns Parsed JSON response typed as T
 * @throws ApiError on non-2xx responses
 */
export async function apiFetch<T = unknown>(
  path: string,
  options: ApiFetchOptions = {},
): Promise<T> {
  const { skipAuth = false, headers: customHeaders, ...rest } = options;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };

  if (!skipAuth) {
    const session = getSession();
    if (session?.token) {
      headers['Authorization'] = `Bearer ${session.token}`;
    }
  }

  if (customHeaders) {
    const h = new Headers(customHeaders as HeadersInit);
    h.forEach((value, key) => {
      headers[key] = value;
    });
  }

  const url = path.startsWith('http') ? path : `${API_BASE_URL}${path}`;

  const response = await fetch(url, {
    ...rest,
    headers,
  });

  if (!response.ok) {
    let body: unknown;
    let message = `Request failed: ${response.status} ${response.statusText}`;
    try {
      body = await response.json();
      if (
        body &&
        typeof body === 'object' &&
        'message' in body &&
        typeof (body as Record<string, unknown>).message === 'string'
      ) {
        message = (body as Record<string, string>).message;
      } else if (
        body &&
        typeof body === 'object' &&
        'error' in body &&
        typeof (body as Record<string, unknown>).error === 'string'
      ) {
        message = (body as Record<string, string>).error;
      }
    } catch {
      // Use default message if body is not JSON
    }
    throw new ApiError(response.status, message, body);
  }

  // 204 No Content — return empty object
  if (response.status === 204) {
    return {} as T;
  }

  return response.json() as Promise<T>;
}

/**
 * Typed fetch wrapper for binary API responses (e.g. a rendered PDF or
 * DOCX export). Mirrors apiFetch's auth-header injection and ApiError
 * handling exactly, but returns the response body as a Blob instead of
 * parsing it as JSON — apiFetch's unconditional `response.json()` call
 * would fail (or silently corrupt) on binary content.
 *
 * @param path - API path, e.g. "/api/v1/cases/:id/report/export"
 * @param options - Standard fetch options plus `skipAuth`
 * @returns The response body as a Blob, with its declared MIME type
 * @throws ApiError on non-2xx responses
 */
export async function apiFetchBlob(
  path: string,
  options: ApiFetchOptions = {},
): Promise<Blob> {
  const { skipAuth = false, headers: customHeaders, ...rest } = options;

  const headers: Record<string, string> = {};

  if (!skipAuth) {
    const session = getSession();
    if (session?.token) {
      headers['Authorization'] = `Bearer ${session.token}`;
    }
  }

  if (customHeaders) {
    const h = new Headers(customHeaders as HeadersInit);
    h.forEach((value, key) => {
      headers[key] = value;
    });
  }

  const url = path.startsWith('http') ? path : `${API_BASE_URL}${path}`;

  const response = await fetch(url, {
    ...rest,
    headers,
  });

  if (!response.ok) {
    let body: unknown;
    let message = `Request failed: ${response.status} ${response.statusText}`;
    try {
      body = await response.json();
      if (
        body &&
        typeof body === 'object' &&
        'message' in body &&
        typeof (body as Record<string, unknown>).message === 'string'
      ) {
        message = (body as Record<string, string>).message;
      } else if (
        body &&
        typeof body === 'object' &&
        'error' in body &&
        typeof (body as Record<string, unknown>).error === 'string'
      ) {
        message = (body as Record<string, string>).error;
      }
    } catch {
      // Use default message if body is not JSON
    }
    throw new ApiError(response.status, message, body);
  }

  return response.blob();
}
