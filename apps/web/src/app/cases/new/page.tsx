'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { StepIndicator } from '@/components/ui/StepIndicator';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Disclaimer } from '@/components/Disclaimer';
import { CaseCreationForm } from '@/components/ingestion/CaseCreationForm';
import { FileUploadPanel } from '@/components/ingestion/FileUploadPanel';
import { DiscardConfirmationBanner } from '@/components/ingestion/DiscardConfirmationBanner';
import { IngestionStatusPanel } from '@/components/ingestion/IngestionStatusPanel';
import { ExtractedTextReview } from '@/components/ingestion/ExtractedTextReview';
import { ClassificationCorrectionPanel } from '@/components/ingestion/ClassificationCorrectionPanel';
import { PartyTimelineEditor } from '@/components/ingestion/PartyTimelineEditor';
import { validateFile, allUploadsSettled, hasNoFailedUploads } from '@/components/ingestion/validation';
import type {
  IngestionStatus,
  SegmentClassification,
  SegmentReview,
  TimelineEvent,
  TimelineParty,
  UploadedFile,
} from '@/types';

const WIZARD_STEPS = [
  { id: 'case', label: 'Case' },
  { id: 'upload', label: 'Upload' },
  { id: 'processing', label: 'Processing' },
  { id: 'review', label: 'Review' },
];

function newFileId(): string {
  return `file-${Math.random().toString(36).slice(2, 10)}`;
}

export default function NewCasePage() {
  const router = useRouter();
  const [stepIndex, setStepIndex] = useState(0);
  const [caseId, setCaseId] = useState<string | null>(null);
  const [files, setFiles] = useState<UploadedFile[]>([]);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [status] = useState<IngestionStatus | null>(null);
  const [segments] = useState<SegmentReview[]>([]);
  const [classifications, setClassifications] = useState<SegmentClassification[]>([]);
  const [parties, setParties] = useState<TimelineParty[]>([]);
  const [events, setEvents] = useState<TimelineEvent[]>([]);

  const handleCaseCreated = (id: string) => {
    setCaseId(id);
    setStepIndex(1);
  };

  const handleFilesAdded = (added: File[]) => {
    setUploadError(null);
    const next: UploadedFile[] = added.map((file) => {
      const error = validateFile(file);
      return {
        id: newFileId(),
        name: file.name,
        size: file.size,
        mimeType: file.type || 'application/octet-stream',
        status: error ? 'failed' : 'queued',
        progress: error ? 0 : 0,
        error: error ?? undefined,
      };
    });
    setFiles((prev) => [...prev, ...next]);
    if (next.some((f) => f.status === 'failed')) {
      setUploadError('One or more files could not be queued. See details below.');
    }
  };

  const handleFileRemoved = (id: string) => {
    setFiles((prev) => prev.filter((f) => f.id !== id));
  };

  const canProceedFromUpload =
    files.length > 0 && allUploadsSettled(files) && hasNoFailedUploads(files);

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-10">
      <div>
        <h1 className="text-2xl font-bold text-neutral-800">New Case Intake</h1>
        <p className="mt-1 text-sm text-neutral-500">
          Create a case, attach source materials, and review the extracted draft content
          before it moves forward for judicial review.
        </p>
      </div>

      <Disclaimer />

      <Card>
        <StepIndicator steps={WIZARD_STEPS} currentStepIndex={stepIndex} />
      </Card>

      <Card>
        {stepIndex === 0 && <CaseCreationForm onCreated={handleCaseCreated} />}

        {stepIndex === 1 && (
          <div className="space-y-6">
            <DiscardConfirmationBanner />
            <FileUploadPanel
              files={files}
              onFilesAdded={handleFilesAdded}
              onFileRemoved={handleFileRemoved}
            />
            {uploadError && (
              <p role="alert" className="text-sm text-red-600">
                {uploadError}
              </p>
            )}
            <div className="flex justify-between">
              <Button variant="ghost" onClick={() => setStepIndex(0)}>
                Back
              </Button>
              <Button
                variant="primary"
                disabled={!canProceedFromUpload}
                onClick={() => setStepIndex(2)}
              >
                Continue
              </Button>
            </div>
          </div>
        )}

        {stepIndex === 2 && (
          <div className="space-y-6">
            <IngestionStatusPanel status={status} />
            <div className="flex justify-between">
              <Button variant="ghost" onClick={() => setStepIndex(1)}>
                Back
              </Button>
              <Button variant="primary" onClick={() => setStepIndex(3)}>
                Continue to Review
              </Button>
            </div>
          </div>
        )}

        {stepIndex === 3 && (
          <div className="space-y-8">
            <ExtractedTextReview segments={segments} />
            <ClassificationCorrectionPanel
              classifications={classifications}
              onCorrected={(updated) =>
                setClassifications((prev) =>
                  prev.map((c) => (c.segmentId === updated.segmentId ? updated : c)),
                )
              }
            />
            <PartyTimelineEditor
              parties={parties}
              events={events}
              onPartiesChange={setParties}
              onEventsChange={setEvents}
            />
            <div className="flex justify-between">
              <Button variant="ghost" onClick={() => setStepIndex(2)}>
                Back
              </Button>
              <Button
                variant="primary"
                onClick={() => caseId && router.push(`/dashboard`)}
              >
                Finish
              </Button>
            </div>
          </div>
        )}
      </Card>
    </div>
  );
}
