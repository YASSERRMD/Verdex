/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { PartiesCategoryPanel } from '@/components/workspace/PartiesCategoryPanel';
import type { CaseLifecycle, CaseParty } from '@/types';

const BASE_CASE: CaseLifecycle = {
  id: 'case-1',
  tenantId: 'tenant-1',
  jurisdictionId: 'jur-1',
  categoryId: 'commercial',
  categoryLabel: 'Commercial',
  subcategoryLabel: 'Breach of Contract',
  title: 'Doe v. Acme Corp',
  state: 'active',
  metadata: {},
  metadataVersion: 1,
  createdBy: 'user-1',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

const PARTIES: CaseParty[] = [
  { id: 'party-1', role: 'first_party', name: 'Jane Doe', counsel: 'Smith & Co.' },
  { id: 'party-2', role: 'second_party', name: 'Acme Corp' },
];

describe('PartiesCategoryPanel', () => {
  it('renders category and subcategory', () => {
    render(<PartiesCategoryPanel caseData={BASE_CASE} parties={[]} />);
    expect(screen.getByText('Commercial')).toBeInTheDocument();
    expect(screen.getByText('Breach of Contract')).toBeInTheDocument();
  });

  it('renders each party with name, role, and counsel', () => {
    render(<PartiesCategoryPanel caseData={BASE_CASE} parties={PARTIES} />);
    expect(screen.getByText('Jane Doe')).toBeInTheDocument();
    expect(screen.getByText('First Party')).toBeInTheDocument();
    expect(screen.getByText(/Smith & Co\./)).toBeInTheDocument();
    expect(screen.getByText('Acme Corp')).toBeInTheDocument();
    expect(screen.getByText('Second Party')).toBeInTheDocument();
  });

  it('shows an empty-state message when there are no parties', () => {
    render(<PartiesCategoryPanel caseData={BASE_CASE} parties={[]} />);
    expect(screen.getByText(/no parties recorded for this case yet/i)).toBeInTheDocument();
  });

  it('falls back to "Not yet assigned" when category is missing', () => {
    const caseData: CaseLifecycle = { ...BASE_CASE, categoryId: '', categoryLabel: undefined };
    render(<PartiesCategoryPanel caseData={caseData} parties={[]} />);
    expect(screen.getByText('Not yet assigned')).toBeInTheDocument();
  });
});
