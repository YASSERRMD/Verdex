/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { CaseHeader } from '@/components/workspace/CaseHeader';
import type { CaseLifecycle } from '@/types';

const BASE_CASE: CaseLifecycle = {
  id: 'case-1',
  tenantId: 'tenant-1',
  jurisdictionId: 'jur-1',
  jurisdictionName: 'District Court of Testland',
  categoryId: 'civil',
  categoryLabel: 'Civil',
  subcategoryLabel: 'Contract Dispute',
  title: 'Doe v. Acme Corp',
  reference: 'DC-2026-0042',
  state: 'active',
  metadata: {},
  metadataVersion: 1,
  createdBy: 'user-1',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

describe('CaseHeader', () => {
  it('renders the case title and reference', () => {
    render(<CaseHeader caseData={BASE_CASE} />);
    expect(screen.getByText('Doe v. Acme Corp')).toBeInTheDocument();
    expect(screen.getByText(/DC-2026-0042/)).toBeInTheDocument();
  });

  it('renders the lifecycle state badge with the correct label', () => {
    render(<CaseHeader caseData={BASE_CASE} />);
    expect(screen.getByTestId('case-state-badge')).toHaveTextContent('Active');
  });

  it('renders category, subcategory, and jurisdiction', () => {
    render(<CaseHeader caseData={BASE_CASE} />);
    expect(screen.getByText('Civil')).toBeInTheDocument();
    expect(screen.getByText(/Contract Dispute/)).toBeInTheDocument();
    expect(screen.getByText('District Court of Testland')).toBeInTheDocument();
  });

  it('falls back to raw IDs when labels are absent', () => {
    const caseData: CaseLifecycle = {
      ...BASE_CASE,
      categoryLabel: undefined,
      jurisdictionName: undefined,
      subcategoryLabel: undefined,
      reference: undefined,
    };
    render(<CaseHeader caseData={caseData} />);
    expect(screen.getByText('civil')).toBeInTheDocument();
    expect(screen.getByText('jur-1')).toBeInTheDocument();
  });

  it('shows the correct badge label per lifecycle state', () => {
    const draftCase: CaseLifecycle = { ...BASE_CASE, state: 'under_review' };
    render(<CaseHeader caseData={draftCase} />);
    expect(screen.getByTestId('case-state-badge')).toHaveTextContent('Under Review');
  });
});
