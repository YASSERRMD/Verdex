/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import LoginForm from '@/app/(auth)/login/LoginForm';

// Mock Next.js router
const mockPush = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}));

// Mock the API fetch helper
jest.mock('@/lib/api', () => ({
  apiFetch: jest.fn(),
  ApiError: class ApiError extends Error {
    constructor(public status: number, message: string) {
      super(message);
      this.name = 'ApiError';
    }
  },
}));

// Mock auth session helpers
jest.mock('@/lib/auth', () => ({
  setSession: jest.fn(),
  getSession: jest.fn(() => null),
  clearSession: jest.fn(),
  isAuthenticated: jest.fn(() => false),
  isTokenExpired: jest.fn(() => false),
}));

import { apiFetch } from '@/lib/api';
const mockApiFetch = apiFetch as jest.MockedFunction<typeof apiFetch>;

describe('LoginForm', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders email and password fields and a submit button', () => {
    render(<LoginForm />);
    expect(screen.getByLabelText(/email address/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });

  it('shows a validation error when email is empty on submit', async () => {
    render(<LoginForm />);
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/email is required/i);
    });
  });

  it('shows a validation error when email format is invalid', async () => {
    render(<LoginForm />);
    await userEvent.type(screen.getByLabelText(/email address/i), 'notanemail');
    await userEvent.type(screen.getByLabelText(/password/i), 'secret123');
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/valid email/i);
    });
  });

  it('shows a validation error when password is too short', async () => {
    render(<LoginForm />);
    await userEvent.type(screen.getByLabelText(/email address/i), 'judge@court.gov');
    await userEvent.type(screen.getByLabelText(/password/i), '123');
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/at least 6 characters/i);
    });
  });

  it('calls apiFetch with correct payload on valid submit', async () => {
    mockApiFetch.mockResolvedValueOnce({
      token: 'fake-jwt-token',
      user: { id: '1', name: 'Judge A', email: 'judge@court.gov', roles: ['judge'] },
    });

    render(<LoginForm />);
    await userEvent.type(screen.getByLabelText(/email address/i), 'judge@court.gov');
    await userEvent.type(screen.getByLabelText(/password/i), 'secret123');
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/api/v1/auth/login',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ email: 'judge@court.gov', password: 'secret123' }),
        }),
      );
    });
  });

  it('shows an error message when the API returns an error', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Invalid credentials'));

    render(<LoginForm />);
    await userEvent.type(screen.getByLabelText(/email address/i), 'judge@court.gov');
    await userEvent.type(screen.getByLabelText(/password/i), 'wrongpassword');
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/invalid credentials/i);
    });
  });

  it('redirects to /dashboard on successful login', async () => {
    mockApiFetch.mockResolvedValueOnce({
      token: 'fake-jwt-token',
      user: { id: '1', name: 'Judge A', email: 'judge@court.gov', roles: ['judge'] },
    });

    render(<LoginForm />);
    await userEvent.type(screen.getByLabelText(/email address/i), 'judge@court.gov');
    await userEvent.type(screen.getByLabelText(/password/i), 'secret123');
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/dashboard');
    });
  });
});
