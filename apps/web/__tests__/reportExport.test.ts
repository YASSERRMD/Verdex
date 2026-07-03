/**
 * @jest-environment jsdom
 */
import { exportCaseReport } from '@/lib/reportExport';
import { ApiError } from '@/lib/api';

describe('exportCaseReport', () => {
  let createObjectURL: jest.Mock;
  let revokeObjectURL: jest.Mock;
  let clickSpy: jest.SpyInstance;
  let fetchMock: jest.Mock;

  beforeEach(() => {
    createObjectURL = jest.fn(() => 'blob:mock-url');
    revokeObjectURL = jest.fn();
    URL.createObjectURL = createObjectURL;
    URL.revokeObjectURL = revokeObjectURL;
    clickSpy = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    fetchMock = jest.fn();
    global.fetch = fetchMock as unknown as typeof fetch;
  });

  afterEach(() => {
    clickSpy.mockRestore();
    jest.restoreAllMocks();
  });

  it('requests the pdf format and downloads the response as a .pdf file', async () => {
    const pdfBlob = new Blob(['%PDF-1.7 fake pdf bytes'], { type: 'application/pdf' });
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      blob: () => Promise.resolve(pdfBlob),
    });

    let capturedDownloadName: string | null = null;
    clickSpy.mockImplementation(function (this: HTMLAnchorElement) {
      capturedDownloadName = this.download;
    });

    await exportCaseReport({ caseId: 'case-1', format: 'pdf', redact: false });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toContain('/api/v1/cases/case-1/report/export');
    expect(url).toContain('format=pdf');
    expect(url).toContain('redact=false');
    // fetch's Headers normalizes header names to lowercase.
    expect((options.headers as Record<string, string>).accept).toBe('application/pdf');

    expect(capturedDownloadName).toBe('case-report-case-1.pdf');
    expect(createObjectURL).toHaveBeenCalledWith(pdfBlob);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:mock-url');
  });

  it('requests the docx format with redact=true and downloads a .docx file', async () => {
    const docxBlob = new Blob(['PK fake docx bytes'], {
      type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    });
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      blob: () => Promise.resolve(docxBlob),
    });

    let capturedDownloadName: string | null = null;
    clickSpy.mockImplementation(function (this: HTMLAnchorElement) {
      capturedDownloadName = this.download;
    });

    await exportCaseReport({ caseId: 'case-2', format: 'docx', redact: true });

    const [url] = fetchMock.mock.calls[0];
    expect(url).toContain('format=docx');
    expect(url).toContain('redact=true');
    expect(capturedDownloadName).toBe('case-report-case-2.docx');
  });

  it('throws an ApiError and does not trigger a download on a non-2xx response', async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 403,
      statusText: 'Forbidden',
      json: () => Promise.resolve({ message: 'actor lacks permission to export this case' }),
    });

    await expect(
      exportCaseReport({ caseId: 'case-1', format: 'pdf', redact: false }),
    ).rejects.toBeInstanceOf(ApiError);

    expect(createObjectURL).not.toHaveBeenCalled();
  });
});
