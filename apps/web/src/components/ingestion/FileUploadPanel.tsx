'use client';

import { useCallback, useRef, useState, type DragEvent } from 'react';
import clsx from 'clsx';
import { UploadIcon, FileIcon, FileAudioIcon, XIcon, CheckCircle2Icon, AlertCircleIcon } from 'lucide-react';
import type { UploadedFile, UploadStatus } from '@/types';

export interface FileUploadPanelProps {
  files: UploadedFile[];
  onFilesAdded: (files: File[]) => void;
  onFileRemoved: (id: string) => void;
  className?: string;
}

const STATUS_LABEL: Record<UploadStatus, string> = {
  queued: 'Queued',
  uploading: 'Uploading',
  uploaded: 'Uploaded',
  failed: 'Failed',
};

const STATUS_CLASSES: Record<UploadStatus, string> = {
  queued: 'bg-neutral-100 text-neutral-600',
  uploading: 'bg-primary-50 text-primary-700',
  uploaded: 'bg-green-50 text-green-700',
  failed: 'bg-red-50 text-red-700',
};

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function isAudioFile(mimeType: string): boolean {
  return mimeType.startsWith('audio/');
}

function StatusChip({ status }: { status: UploadStatus }) {
  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium',
        STATUS_CLASSES[status],
      )}
    >
      {status === 'uploaded' && <CheckCircle2Icon className="h-3 w-3" aria-hidden="true" />}
      {status === 'failed' && <AlertCircleIcon className="h-3 w-3" aria-hidden="true" />}
      {STATUS_LABEL[status]}
    </span>
  );
}

export function FileUploadPanel({
  files,
  onFilesAdded,
  onFileRemoved,
  className,
}: FileUploadPanelProps) {
  const [isDragging, setIsDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleDragOver = useCallback((e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);
  }, []);

  const handleDrop = useCallback(
    (e: DragEvent<HTMLDivElement>) => {
      e.preventDefault();
      setIsDragging(false);
      const dropped = Array.from(e.dataTransfer.files ?? []);
      if (dropped.length > 0) onFilesAdded(dropped);
    },
    [onFilesAdded],
  );

  const handlePick = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const picked = Array.from(e.target.files ?? []);
      if (picked.length > 0) onFilesAdded(picked);
      // Reset so the same file can be re-selected after removal.
      e.target.value = '';
    },
    [onFilesAdded],
  );

  return (
    <div className={clsx('space-y-4', className)}>
      <div>
        <h2 className="text-lg font-semibold text-neutral-800">Upload Documents &amp; Audio</h2>
        <p className="mt-1 text-sm text-neutral-500">
          Attach case documents and audio recordings. Files are hashed for provenance, then
          transcribed or text-extracted for review.
        </p>
      </div>

      <div
        role="button"
        tabIndex={0}
        aria-label="Upload files by dropping them here or choosing files"
        data-testid="dropzone"
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={() => inputRef.current?.click()}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            inputRef.current?.click();
          }
        }}
        className={clsx(
          'flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-6 py-10 text-center transition-colors cursor-pointer',
          isDragging
            ? 'border-primary-DEFAULT bg-primary-50'
            : 'border-neutral-300 bg-neutral-50 hover:bg-neutral-100',
        )}
      >
        <UploadIcon className="h-8 w-8 text-neutral-400" aria-hidden="true" />
        <p className="text-sm font-medium text-neutral-700">
          Drag and drop files here, or click to browse
        </p>
        <p className="text-xs text-neutral-500">
          Supports documents and audio recordings. Multiple files allowed.
        </p>
        <input
          ref={inputRef}
          type="file"
          multiple
          className="sr-only"
          aria-label="Choose files to upload"
          onChange={handlePick}
        />
      </div>

      {files.length === 0 ? (
        <p className="text-sm text-neutral-400">No files queued yet.</p>
      ) : (
        <ul className="divide-y divide-neutral-200 rounded-lg border border-neutral-200 bg-white">
          {files.map((file) => (
            <li key={file.id} className="flex items-center gap-3 px-4 py-3">
              {isAudioFile(file.mimeType) ? (
                <FileAudioIcon className="h-5 w-5 flex-shrink-0 text-neutral-400" aria-hidden="true" />
              ) : (
                <FileIcon className="h-5 w-5 flex-shrink-0 text-neutral-400" aria-hidden="true" />
              )}
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-neutral-800">{file.name}</p>
                <p className="text-xs text-neutral-500">{formatBytes(file.size)}</p>
                {file.status === 'failed' && file.error && (
                  <p role="alert" className="mt-0.5 text-xs text-red-600">
                    {file.error}
                  </p>
                )}
              </div>
              <StatusChip status={file.status} />
              <button
                type="button"
                aria-label={`Remove ${file.name}`}
                onClick={() => onFileRemoved(file.id)}
                className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-600"
              >
                <XIcon className="h-4 w-4" aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
