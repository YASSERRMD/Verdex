/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { IngestionStatusPanel } from '@/components/ingestion/IngestionStatusPanel';
import type { IngestionStatus } from '@/types';

describe('IngestionStatusPanel', () => {
  it('renders an empty-state message when there is no status', () => {
    render(<IngestionStatusPanel status={null} />);
    expect(screen.getByText(/no ingestion job in progress/i)).toBeInTheDocument();
  });

  it('renders the current stage label and progress percentage', () => {
    const status: IngestionStatus = {
      jobId: 'job-1',
      stage: 'extraction',
      percentComplete: 40,
    };
    render(<IngestionStatusPanel status={status} />);
    expect(screen.getAllByText(/transcribing \/ extracting text/i).length).toBeGreaterThan(0);
    expect(screen.getByText('40%')).toBeInTheDocument();
  });

  it('reflects percentComplete in the progress bar width', () => {
    const status: IngestionStatus = {
      jobId: 'job-1',
      stage: 'segment',
      percentComplete: 75,
    };
    render(<IngestionStatusPanel status={status} />);
    const fill = screen.getByTestId('progress-bar-fill');
    expect(fill).toHaveStyle({ width: '75%' });
  });

  it('shows a success state when the stage is complete', () => {
    const status: IngestionStatus = {
      jobId: 'job-1',
      stage: 'complete',
      percentComplete: 100,
    };
    render(<IngestionStatusPanel status={status} />);
    expect(screen.getAllByText(/processing complete/i).length).toBeGreaterThan(0);
  });

  it('shows an error state and message when the stage has failed', () => {
    const status: IngestionStatus = {
      jobId: 'job-1',
      stage: 'failed',
      percentComplete: 0,
      error: 'Transcription service unavailable',
    };
    render(<IngestionStatusPanel status={status} />);
    expect(screen.getByText(/processing failed/i)).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent(/transcription service unavailable/i);
  });

  it('does not imply a verdict or final decision in its messaging', () => {
    const status: IngestionStatus = {
      jobId: 'job-1',
      stage: 'classify',
      percentComplete: 90,
    };
    render(<IngestionStatusPanel status={status} />);
    expect(screen.getByTestId('ingestion-status-panel')).not.toHaveTextContent(
      /verdict|ruling|decision/i,
    );
  });
});
