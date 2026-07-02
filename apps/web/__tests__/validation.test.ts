import {
  validateCaseCreationInput,
  validateFile,
  hasNoFailedUploads,
  allUploadsSettled,
  validateParty,
  validateEvent,
  MAX_FILE_SIZE_BYTES,
} from '@/components/ingestion/validation';
import type { CaseCreationInput, TimelineEvent, TimelineParty, UploadedFile } from '@/types';

describe('validateCaseCreationInput', () => {
  it('returns errors for all missing required fields', () => {
    const input: CaseCreationInput = { category: '', firstPartyName: '', secondPartyName: '' };
    const errors = validateCaseCreationInput(input);
    expect(errors.category).toBeDefined();
    expect(errors.firstPartyName).toBeDefined();
    expect(errors.secondPartyName).toBeDefined();
  });

  it('returns no errors when all fields are valid', () => {
    const input: CaseCreationInput = {
      category: 'civil',
      firstPartyName: 'Jane Doe',
      secondPartyName: 'Acme Corp',
    };
    expect(validateCaseCreationInput(input)).toEqual({});
  });

  it('treats whitespace-only party names as invalid', () => {
    const input: CaseCreationInput = {
      category: 'civil',
      firstPartyName: '   ',
      secondPartyName: 'Acme Corp',
    };
    const errors = validateCaseCreationInput(input);
    expect(errors.firstPartyName).toBeDefined();
  });
});

describe('validateFile', () => {
  it('rejects empty files', () => {
    const file = new File([], 'empty.pdf');
    Object.defineProperty(file, 'size', { value: 0 });
    expect(validateFile(file)).toMatch(/empty/i);
  });

  it('rejects files over the max size', () => {
    const file = new File(['x'], 'huge.mp3');
    Object.defineProperty(file, 'size', { value: MAX_FILE_SIZE_BYTES + 1 });
    expect(validateFile(file)).toMatch(/exceeds/i);
  });

  it('accepts a normal-sized file', () => {
    const file = new File(['hello world'], 'doc.pdf');
    expect(validateFile(file)).toBeNull();
  });
});

describe('hasNoFailedUploads / allUploadsSettled', () => {
  const base: Omit<UploadedFile, 'status'> = {
    id: '1',
    name: 'a.pdf',
    size: 10,
    mimeType: 'application/pdf',
    progress: 0,
  };

  it('hasNoFailedUploads returns false when any file has failed', () => {
    const files: UploadedFile[] = [{ ...base, status: 'failed' }];
    expect(hasNoFailedUploads(files)).toBe(false);
  });

  it('hasNoFailedUploads returns true when no file has failed', () => {
    const files: UploadedFile[] = [{ ...base, status: 'uploaded' }];
    expect(hasNoFailedUploads(files)).toBe(true);
  });

  it('allUploadsSettled returns false while a file is still uploading', () => {
    const files: UploadedFile[] = [{ ...base, status: 'uploading' }];
    expect(allUploadsSettled(files)).toBe(false);
  });

  it('allUploadsSettled returns true once every file is uploaded or failed', () => {
    const files: UploadedFile[] = [
      { ...base, id: '1', status: 'uploaded' },
      { ...base, id: '2', status: 'failed' },
    ];
    expect(allUploadsSettled(files)).toBe(true);
  });
});

describe('validateParty', () => {
  it('requires a non-empty name', () => {
    const party: TimelineParty = { id: 'p1', role: 'first_party', name: '  ' };
    expect(validateParty(party)).toMatch(/required/i);
  });

  it('accepts a party with a name', () => {
    const party: TimelineParty = { id: 'p1', role: 'first_party', name: 'Jane Doe' };
    expect(validateParty(party)).toBeNull();
  });
});

describe('validateEvent', () => {
  it('requires a non-empty description', () => {
    const event: TimelineEvent = { id: 'e1', description: '' };
    expect(validateEvent(event)).toMatch(/required/i);
  });

  it('rejects an invalid date string', () => {
    const event: TimelineEvent = { id: 'e1', description: 'Filed complaint', occurredAt: 'not-a-date' };
    expect(validateEvent(event)).toMatch(/invalid/i);
  });

  it('accepts a valid event with a valid date', () => {
    const event: TimelineEvent = {
      id: 'e1',
      description: 'Filed complaint',
      occurredAt: '2024-03-15',
    };
    expect(validateEvent(event)).toBeNull();
  });
});
