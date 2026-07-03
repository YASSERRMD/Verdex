/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { CaseloadPanel } from '@/components/dashboard/CaseloadPanel';
import { CategoryBreakdownPanel } from '@/components/dashboard/CategoryBreakdownPanel';
import { JurisdictionBreakdownPanel } from '@/components/dashboard/JurisdictionBreakdownPanel';
import { QualityTrendPanel } from '@/components/dashboard/QualityTrendPanel';
import { UsageCostPanel } from '@/components/dashboard/UsageCostPanel';
import { apiFetch } from '@/lib/api';
import type { AnalyticsMetrics, QualityTrend, UsageDashboard } from '@/types';

jest.mock('@/lib/api', () => ({
  apiFetch: jest.fn(),
}));

const mockedApiFetch = apiFetch as jest.MockedFunction<typeof apiFetch>;

afterEach(() => {
  jest.clearAllMocks();
});

const METRICS: AnalyticsMetrics = {
  tenantId: 'tenant-1',
  generatedAt: '2026-07-01T00:00:00Z',
  totalCases: 6,
  byState: [
    { state: 'active', count: 3 },
    { state: 'under_review', count: 1 },
    { state: 'closed', count: 2 },
  ],
  byCategory: [
    { categoryId: 'contract', count: 4 },
    { categoryId: 'tort', count: 2 },
  ],
  byJurisdiction: [
    {
      jurisdictionId: 'jur-1',
      count: 6,
      byState: [
        { state: 'active', count: 3 },
        { state: 'closed', count: 2 },
        { state: 'under_review', count: 1 },
      ],
    },
  ],
  createdTrend: [
    { date: '2026-06-30', count: 4 },
    { date: '2026-07-01', count: 2 },
  ],
};

const QUALITY_TREND: QualityTrend = {
  points: [
    { jurisdictionCode: 'AE-DXB', legalFamily: 'civil_law', count: 5, avgOverall: 0.82, avgPerDimension: {} },
    { jurisdictionCode: 'US-NY', count: 2, avgOverall: 0.65, avgPerDimension: {} },
  ],
};

const USAGE: UsageDashboard = {
  tenantId: 'tenant-1',
  generatedAt: '2026-07-01T00:00:00Z',
  byProvider: [
    {
      providerId: 'anthropic',
      totalInputTokens: 1000,
      totalOutputTokens: 500,
      totalTokens: 1500,
      estimatedCostUsd: 4.5,
      requestCount: 10,
    },
  ],
  byTaskType: [],
  last7DaysTrend: [],
  totalTokens: 1500,
  estimatedCostUsd: 4.5,
  requestCount: 10,
};

describe('CaseloadPanel', () => {
  it('renders total and per-state counts from the caseload endpoint', async () => {
    mockedApiFetch.mockResolvedValueOnce(METRICS);
    render(<CaseloadPanel />);

    await waitFor(() =>
      expect(mockedApiFetch).toHaveBeenCalledWith('/api/v1/analytics/caseload'),
    );
    expect(await screen.findByText('6')).toBeInTheDocument();
    expect(screen.getByText('Total Cases')).toBeInTheDocument();
  });

  it('shows an error state when the request fails', async () => {
    mockedApiFetch.mockRejectedValueOnce(new Error('boom'));
    render(<CaseloadPanel />);

    expect(await screen.findByTestId('caseload-panel-error')).toBeInTheDocument();
  });
});

describe('CategoryBreakdownPanel', () => {
  it('renders a row per category', async () => {
    mockedApiFetch.mockResolvedValueOnce(METRICS);
    render(<CategoryBreakdownPanel />);

    expect(await screen.findByTestId('category-row-contract')).toBeInTheDocument();
    expect(screen.getByTestId('category-row-tort')).toBeInTheDocument();
  });

  it('shows an empty state with no categories', async () => {
    mockedApiFetch.mockResolvedValueOnce({ ...METRICS, byCategory: [] });
    render(<CategoryBreakdownPanel />);

    expect(await screen.findByTestId('category-breakdown-empty')).toBeInTheDocument();
  });
});

describe('JurisdictionBreakdownPanel', () => {
  it('renders a row per jurisdiction with its state breakdown', async () => {
    mockedApiFetch.mockResolvedValueOnce(METRICS);
    render(<JurisdictionBreakdownPanel />);

    const row = await screen.findByTestId('jurisdiction-row-jur-1');
    expect(row).toBeInTheDocument();
    expect(row).toHaveTextContent('active: 3');
  });
});

describe('QualityTrendPanel', () => {
  it('renders a row per jurisdiction quality trend point', async () => {
    mockedApiFetch.mockResolvedValueOnce(QUALITY_TREND);
    render(<QualityTrendPanel />);

    expect(await screen.findByTestId('quality-row-AE-DXB')).toBeInTheDocument();
    expect(screen.getByTestId('quality-row-US-NY')).toBeInTheDocument();
  });

  it('shows an empty state with no scores', async () => {
    mockedApiFetch.mockResolvedValueOnce({ points: [] });
    render(<QualityTrendPanel />);

    expect(await screen.findByTestId('quality-trend-empty')).toBeInTheDocument();
  });
});

describe('UsageCostPanel', () => {
  it('does not render for roles without audit permission', () => {
    render(<UsageCostPanel roles={['viewer']} />);

    expect(mockedApiFetch).not.toHaveBeenCalled();
    expect(screen.queryByTestId('usage-cost-panel')).not.toBeInTheDocument();
  });

  it('fetches and renders usage data for admin/judge roles', async () => {
    mockedApiFetch.mockResolvedValueOnce(USAGE);
    render(<UsageCostPanel roles={['judge']} />);

    await waitFor(() =>
      expect(mockedApiFetch).toHaveBeenCalledWith('/api/v1/analytics/usage'),
    );
    expect(await screen.findByTestId('usage-cost-panel')).toBeInTheDocument();
    expect(screen.getByText('$4.50')).toBeInTheDocument();
    expect(screen.getByTestId('usage-provider-anthropic')).toBeInTheDocument();
  });

  it('shows an error state when the server forbids the request', async () => {
    mockedApiFetch.mockRejectedValueOnce(new Error('forbidden'));
    render(<UsageCostPanel roles={['admin']} />);

    expect(await screen.findByTestId('usage-cost-panel-error')).toBeInTheDocument();
  });
});
