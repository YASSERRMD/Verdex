import type { CaseCreationInput, TimelineEvent, TimelineParty, UploadedFile } from '@/types';

export type FieldErrors<T> = Partial<Record<keyof T, string>>;

/**
 * Validates case-creation form input. Returns an empty object when valid.
 */
export function validateCaseCreationInput(
  value: CaseCreationInput,
): FieldErrors<CaseCreationInput> {
  const errors: FieldErrors<CaseCreationInput> = {};
  if (!value.category) {
    errors.category = 'Please select a case category.';
  }
  if (!value.firstPartyName.trim()) {
    errors.firstPartyName = 'First party name is required.';
  }
  if (!value.secondPartyName.trim()) {
    errors.secondPartyName = 'Second party name is required.';
  }
  return errors;
}

/** Maximum single-file size accepted by the upload panel (100 MB). */
export const MAX_FILE_SIZE_BYTES = 100 * 1024 * 1024;

/**
 * Validates a single File before it is queued for upload. Returns an error
 * message, or null when the file is acceptable.
 */
export function validateFile(file: File): string | null {
  if (file.size === 0) {
    return 'File is empty.';
  }
  if (file.size > MAX_FILE_SIZE_BYTES) {
    return 'File exceeds the 100 MB upload limit.';
  }
  return null;
}

/**
 * Returns true if the queued/uploading/uploaded file list has no files in a
 * failed state — used to gate progression to the next ingestion step.
 */
export function hasNoFailedUploads(files: UploadedFile[]): boolean {
  return files.every((f) => f.status !== 'failed');
}

/**
 * Returns true if there is at least one file that is not still in-flight
 * (queued or uploading) — used to determine whether upload is "settled".
 */
export function allUploadsSettled(files: UploadedFile[]): boolean {
  return files.every((f) => f.status === 'uploaded' || f.status === 'failed');
}

/** Validates a single timeline party. Returns an error message, or null. */
export function validateParty(party: TimelineParty): string | null {
  if (!party.name.trim()) {
    return 'Party name is required.';
  }
  return null;
}

/** Validates a single timeline event. Returns an error message, or null. */
export function validateEvent(event: TimelineEvent): string | null {
  if (!event.description.trim()) {
    return 'Event description is required.';
  }
  if (event.occurredAt) {
    const parsed = Date.parse(event.occurredAt);
    if (Number.isNaN(parsed)) {
      return 'Event date is invalid.';
    }
  }
  return null;
}
