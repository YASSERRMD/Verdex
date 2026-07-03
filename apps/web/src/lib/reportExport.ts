import { apiFetchBlob } from '@/lib/api';

/**
 * The server-rendered export formats packages/reportexport supports.
 * Markdown/Text exports stay client-side (see opinionExport.ts) since
 * the browser already has everything needed to render them from data
 * already on screen; PDF and DOCX require the real Go renderers
 * (gofpdf / archive+zip OOXML) so those two always go through the API.
 */
export type ReportExportFormat = 'pdf' | 'docx';

const CONTENT_TYPE_BY_FORMAT: Record<ReportExportFormat, string> = {
  pdf: 'application/pdf',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
};

const EXTENSION_BY_FORMAT: Record<ReportExportFormat, string> = {
  pdf: 'pdf',
  docx: 'docx',
};

/**
 * Triggers a browser download of a Blob as `filename`. Mirrors
 * opinionExport.ts's `triggerDownload` exactly (same
 * jsdom/URL.createObjectURL-stubbing rationale), adapted to accept an
 * already-fetched Blob rather than building one from string content.
 */
function triggerBlobDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  try {
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
  } finally {
    URL.revokeObjectURL(url);
  }
}

export interface ExportReportOptions {
  /** The case this report is being exported for. */
  caseId: string;
  /** Which server-rendered format to request. */
  format: ReportExportFormat;
  /**
   * When true, requests the PII-redacted rendering (reportexport.Redact
   * applied server-side before rendering) instead of the unredacted
   * report.
   */
  redact: boolean;
}

/**
 * Requests a server-rendered case report export
 * (packages/reportexport.Service.Export, via the API's report-export
 * endpoint) and triggers a browser download of the resulting file.
 *
 * The endpoint is expected to return the raw rendered bytes
 * (`%PDF-`-prefixed for pdf, a zip archive for docx) with the
 * corresponding Content-Type; every call is recorded server-side in
 * reportexport's export audit trail (who, when, format,
 * redaction-on/off), independent of anything tracked client-side.
 */
export async function exportCaseReport({
  caseId,
  format,
  redact,
}: ExportReportOptions): Promise<void> {
  const query = new URLSearchParams({ format, redact: String(redact) });
  const blob = await apiFetchBlob(
    `/api/v1/cases/${encodeURIComponent(caseId)}/report/export?${query.toString()}`,
    { headers: { Accept: CONTENT_TYPE_BY_FORMAT[format] } },
  );
  triggerBlobDownload(blob, `case-report-${caseId}.${EXTENSION_BY_FORMAT[format]}`);
}
