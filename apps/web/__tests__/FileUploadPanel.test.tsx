/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { FileUploadPanel } from '@/components/ingestion/FileUploadPanel';
import type { UploadedFile } from '@/types';

describe('FileUploadPanel', () => {
  it('renders the dropzone and an empty-state message when there are no files', () => {
    render(
      <FileUploadPanel files={[]} onFilesAdded={jest.fn()} onFileRemoved={jest.fn()} />,
    );
    expect(screen.getByTestId('dropzone')).toBeInTheDocument();
    expect(screen.getByText(/no files queued yet/i)).toBeInTheDocument();
  });

  it('calls onFilesAdded when files are picked via the file input', () => {
    const onFilesAdded = jest.fn();
    render(
      <FileUploadPanel files={[]} onFilesAdded={onFilesAdded} onFileRemoved={jest.fn()} />,
    );
    const input = screen.getByLabelText(/choose files to upload/i) as HTMLInputElement;
    const file = new File(['hello'], 'evidence.pdf', { type: 'application/pdf' });
    fireEvent.change(input, { target: { files: [file] } });
    expect(onFilesAdded).toHaveBeenCalledWith([file]);
  });

  it('calls onFilesAdded when files are dropped on the dropzone', () => {
    const onFilesAdded = jest.fn();
    render(
      <FileUploadPanel files={[]} onFilesAdded={onFilesAdded} onFileRemoved={jest.fn()} />,
    );
    const dropzone = screen.getByTestId('dropzone');
    const file = new File(['audio'], 'hearing.mp3', { type: 'audio/mpeg' });
    fireEvent.drop(dropzone, { dataTransfer: { files: [file] } });
    expect(onFilesAdded).toHaveBeenCalledWith([file]);
  });

  it('renders a status chip for each queued file', () => {
    const files: UploadedFile[] = [
      {
        id: '1',
        name: 'contract.pdf',
        size: 2048,
        mimeType: 'application/pdf',
        status: 'uploaded',
        progress: 100,
      },
      {
        id: '2',
        name: 'call.mp3',
        size: 4096,
        mimeType: 'audio/mpeg',
        status: 'failed',
        progress: 0,
        error: 'Upload failed',
      },
    ];
    render(<FileUploadPanel files={files} onFilesAdded={jest.fn()} onFileRemoved={jest.fn()} />);
    expect(screen.getByText('contract.pdf')).toBeInTheDocument();
    expect(screen.getByText('Uploaded')).toBeInTheDocument();
    expect(screen.getByText('call.mp3')).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.getByText('Upload failed')).toBeInTheDocument();
  });

  it('calls onFileRemoved when the remove button is clicked', () => {
    const onFileRemoved = jest.fn();
    const files: UploadedFile[] = [
      {
        id: '1',
        name: 'contract.pdf',
        size: 2048,
        mimeType: 'application/pdf',
        status: 'queued',
        progress: 0,
      },
    ];
    render(
      <FileUploadPanel files={files} onFilesAdded={jest.fn()} onFileRemoved={onFileRemoved} />,
    );
    fireEvent.click(screen.getByLabelText(/remove contract.pdf/i));
    expect(onFileRemoved).toHaveBeenCalledWith('1');
  });
});
