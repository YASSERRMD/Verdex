'use client';

import { useState } from 'react';
import clsx from 'clsx';
import {
  AlertOctagonIcon,
  FileJsonIcon,
  FileTextIcon,
  GitBranchIcon,
  MessageSquarePlusIcon,
  ScaleIcon,
} from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Disclaimer } from '@/components/Disclaimer';
import {
  exportOpinionAsJSON,
  exportOpinionAsMarkdown,
  exportOpinionAsText,
} from '@/lib/opinionExport';
import type {
  CaseOpinion,
  IssueOpinion,
  OpinionArgument,
  OpinionComment,
  OpinionPartyRole,
} from '@/types';

export interface ReasoningOpinionPanelProps {
  /** True while a draft opinion is being synthesized. */
  loading?: boolean;
  /**
   * True once a draft opinion is known to exist but its full content has
   * not been supplied via `opinion` yet — kept for backward compatibility
   * with callers that only know draft *existence*, not its content.
   */
  hasDraftOpinion?: boolean;
  /**
   * The case's full draft reasoned opinion. When provided, the panel
   * renders the complete per-issue review UI (arguments, evidence weights,
   * uncertainty callouts, conclusion, trace links, comments, export
   * controls). Takes precedence over `hasDraftOpinion`.
   */
  opinion?: CaseOpinion | null;
  /**
   * Called when a reviewer asks to view the full supporting trace for one
   * conclusion's fact/rule nodes — the case workspace uses this to switch
   * to the Reasoning Tree tab with the relevant node selected, per Phase
   * 065's TreeVisualizationPanel/TreeNodeDetail pattern, rather than this
   * panel duplicating tree rendering itself.
   */
  onViewTrace?: (issueNodeId: string, nodeId: string) => void;
  className?: string;
}

const PARTY_LABELS: Record<OpinionPartyRole, string> = {
  first_party: 'First Party',
  second_party: 'Second Party',
};

function newCommentId(): string {
  return `comment-${Math.random().toString(36).slice(2, 10)}`;
}

function ArgumentColumn({
  title,
  args,
  onViewTrace,
  testIdPrefix,
}: {
  title: string;
  args: OpinionArgument[];
  onViewTrace?: (nodeId: string) => void;
  testIdPrefix: string;
}) {
  return (
    <div data-testid={testIdPrefix} className="min-w-0 flex-1 space-y-3">
      <h4 className="text-sm font-semibold text-neutral-700 dark:text-neutral-200">{title}</h4>
      {args.length === 0 ? (
        <p className="text-xs text-neutral-400">No arguments recorded for this party.</p>
      ) : (
        <ul className="space-y-2">
          {args.map((argument) => (
            <li
              key={argument.id}
              data-testid={`opinion-argument-${argument.id}`}
              className="rounded-lg border border-neutral-200 px-3 py-2.5 text-sm dark:border-neutral-700"
            >
              <p className="text-neutral-800 dark:text-neutral-100">{argument.claim}</p>
              <div className="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-neutral-500">
                <span data-testid={`opinion-argument-strength-${argument.id}`}>
                  Strength {Math.round(argument.strength * 100)}%
                </span>
                {!argument.grounded && (
                  <span className="rounded-full bg-amber-50 px-2 py-0.5 font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                    Ungrounded reference stripped
                  </span>
                )}
                {(argument.supportingFactIds.length > 0 || argument.supportingRuleIds.length > 0) &&
                  onViewTrace && (
                    <button
                      type="button"
                      onClick={() =>
                        onViewTrace(argument.supportingFactIds[0] ?? argument.supportingRuleIds[0])
                      }
                      className="inline-flex items-center gap-1 font-medium text-primary-DEFAULT hover:underline"
                    >
                      <GitBranchIcon className="h-3 w-3" aria-hidden="true" />
                      View supporting nodes
                    </button>
                  )}
              </div>
              {argument.counterarguments && argument.counterarguments.length > 0 && (
                <p className="mt-1.5 text-xs text-neutral-400">
                  Anticipated counterarguments: {argument.counterarguments.join('; ')}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function IssueSection({
  issue,
  comments,
  onAddComment,
  onViewTrace,
}: {
  issue: IssueOpinion;
  comments: OpinionComment[];
  onAddComment: (issueNodeId: string, text: string) => void;
  onViewTrace?: (issueNodeId: string, nodeId: string) => void;
}) {
  const [commentDraft, setCommentDraft] = useState('');
  const issueComments = comments.filter((c) => c.issueNodeId === issue.issueNodeId);

  const handleSubmitComment = () => {
    const trimmed = commentDraft.trim();
    if (!trimmed) return;
    onAddComment(issue.issueNodeId, trimmed);
    setCommentDraft('');
  };

  const traceHandler = onViewTrace
    ? (nodeId: string) => onViewTrace(issue.issueNodeId, nodeId)
    : undefined;

  const conclusionTraceNodeId =
    issue.conclusion.supportingFactIds[0] ?? issue.conclusion.supportingRuleIds[0];

  return (
    <section
      data-testid={`opinion-issue-${issue.issueNodeId}`}
      className="space-y-4 rounded-xl border border-neutral-200 p-4 dark:border-neutral-700"
    >
      <header>
        <p className="text-xs font-medium uppercase tracking-wide text-neutral-400">Issue</p>
        <h3 className="text-base font-semibold text-neutral-800 dark:text-white">
          {issue.issueText}
        </h3>
      </header>

      {issue.uncertainties.length > 0 && (
        <ul
          data-testid={`opinion-uncertainty-${issue.issueNodeId}`}
          className="space-y-2"
          aria-label="Uncertainty callouts"
        >
          {issue.uncertainties.map((uncertainty, index) => (
            <li
              key={`${uncertainty.source}-${index}`}
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-xs text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
            >
              <AlertOctagonIcon className="mt-0.5 h-4 w-4 flex-shrink-0 text-red-500" aria-hidden="true" />
              <div>
                <p className="font-semibold">
                  {uncertainty.source.replace(/_/g, ' ')} · Severity{' '}
                  {Math.round(uncertainty.severity * 100)}%
                </p>
                <p>{uncertainty.caveat}</p>
              </div>
            </li>
          ))}
        </ul>
      )}

      <div className="flex flex-col gap-4 sm:flex-row">
        <ArgumentColumn
          title={PARTY_LABELS.first_party}
          args={issue.firstPartyArguments}
          onViewTrace={traceHandler}
          testIdPrefix={`opinion-first-party-args-${issue.issueNodeId}`}
        />
        <div className="hidden w-px self-stretch bg-neutral-200 dark:bg-neutral-700 sm:block" aria-hidden="true" />
        <ArgumentColumn
          title={PARTY_LABELS.second_party}
          args={issue.secondPartyArguments}
          onViewTrace={traceHandler}
          testIdPrefix={`opinion-second-party-args-${issue.issueNodeId}`}
        />
      </div>

      {issue.evidenceWeights.length > 0 && (
        <div data-testid={`opinion-evidence-weights-${issue.issueNodeId}`}>
          <h4 className="mb-1.5 text-sm font-semibold text-neutral-700 dark:text-neutral-200">
            Evidence Weights
          </h4>
          <ul className="space-y-1.5">
            {issue.evidenceWeights.map((weight) => (
              <li
                key={weight.factNodeId}
                className="flex flex-wrap items-center justify-between gap-2 rounded-md bg-neutral-50 px-3 py-1.5 text-xs text-neutral-600 dark:bg-neutral-900/40 dark:text-neutral-300"
              >
                <span>{weight.rationale}</span>
                <span className="flex items-center gap-2 font-medium">
                  {weight.contradicted && (
                    <span className="rounded-full bg-red-100 px-2 py-0.5 text-red-700 dark:bg-red-900/40 dark:text-red-300">
                      Contradicted
                    </span>
                  )}
                  Weight {Math.round(weight.weight * 100)}% · {weight.corroborationCount} corroborating
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      <div
        data-testid={`opinion-conclusion-${issue.issueNodeId}`}
        className="rounded-lg border border-primary-DEFAULT/30 bg-primary-50/40 p-3 dark:border-primary-DEFAULT/40 dark:bg-primary-900/10"
      >
        <div className="mb-1 flex items-center justify-between gap-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-primary-DEFAULT">
            Tentative Draft Conclusion
          </p>
          <span className="text-xs text-neutral-500">
            Confidence {Math.round(issue.conclusion.confidence * 100)}%
          </span>
        </div>
        <p className="text-sm text-neutral-800 dark:text-neutral-100">{issue.conclusion.text}</p>
        {issue.conclusion.favoredParty && (
          <p className="mt-1 text-xs text-neutral-500">
            Currently favors: {PARTY_LABELS[issue.conclusion.favoredParty]}
          </p>
        )}
        {issue.conclusion.weakestLink && (
          <p className="mt-2 flex items-start gap-1.5 rounded-md bg-amber-50 px-2.5 py-1.5 text-xs text-amber-800 dark:bg-amber-900/30 dark:text-amber-200">
            <AlertOctagonIcon className="mt-0.5 h-3.5 w-3.5 flex-shrink-0" aria-hidden="true" />
            <span>Weakest link: {issue.conclusion.weakestLink}</span>
          </p>
        )}
        {traceHandler && conclusionTraceNodeId && (
          <button
            type="button"
            onClick={() => traceHandler(conclusionTraceNodeId)}
            className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-primary-DEFAULT hover:underline"
          >
            <GitBranchIcon className="h-3.5 w-3.5" aria-hidden="true" />
            View full trace
          </button>
        )}
      </div>

      <div data-testid={`opinion-comments-${issue.issueNodeId}`} className="space-y-2">
        <h4 className="text-sm font-semibold text-neutral-700 dark:text-neutral-200">
          Judge Comments
        </h4>
        {issueComments.length === 0 ? (
          <p className="text-xs text-neutral-400">No comments yet.</p>
        ) : (
          <ul className="space-y-1.5">
            {issueComments.map((comment) => (
              <li
                key={comment.id}
                data-testid={`opinion-comment-${comment.id}`}
                className="rounded-md border border-neutral-200 px-3 py-2 text-xs dark:border-neutral-700"
              >
                <p className="text-neutral-700 dark:text-neutral-200">{comment.text}</p>
                <p className="mt-1 text-neutral-400">
                  {comment.author} · {new Date(comment.occurredAt).toLocaleString()}
                </p>
              </li>
            ))}
          </ul>
        )}
        <div className="flex items-start gap-2">
          <textarea
            aria-label={`Add a comment for ${issue.issueText}`}
            value={commentDraft}
            onChange={(event) => setCommentDraft(event.target.value)}
            placeholder="Add a comment or annotation…"
            rows={2}
            className="block w-full flex-1 rounded-lg border border-neutral-300 px-3 py-2 text-sm shadow-sm placeholder:text-neutral-400 focus:border-primary-DEFAULT focus:outline-none focus:ring-2 focus:ring-primary-DEFAULT/30 dark:border-neutral-600 dark:bg-neutral-900"
          />
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={!commentDraft.trim()}
            onClick={handleSubmitComment}
            leftIcon={<MessageSquarePlusIcon className="h-3.5 w-3.5" />}
          >
            Add
          </Button>
        </div>
      </div>
    </section>
  );
}

/**
 * The reasoned-opinion review panel: one section per issue with both
 * parties' arguments side by side, inline evidence weights, uncertainty
 * callouts, a trace-link back to the reasoning tree, a judge comment box,
 * and export controls — all beneath the always-rendered non-binding
 * disclaimer (Phase 057 guardrail). No `/api/v1/cases/:caseId/opinion`
 * endpoint exists yet (same situation TreeVisualizationPanel documents for
 * `/tree` in Phase 065) — the case workspace page is expected to fetch and
 * pass down a `CaseOpinion` once that endpoint exists; until then this
 * panel is exercised directly with an `opinion` prop (see tests).
 */
export function ReasoningOpinionPanel({
  loading = false,
  hasDraftOpinion = false,
  opinion,
  onViewTrace,
  className,
}: ReasoningOpinionPanelProps) {
  const [comments, setComments] = useState<OpinionComment[]>([]);

  const handleAddComment = (issueNodeId: string, text: string) => {
    setComments((prev) => [
      ...prev,
      {
        id: newCommentId(),
        issueNodeId,
        text,
        author: 'Current Reviewer',
        occurredAt: new Date().toISOString(),
      },
    ]);
  };

  const hasFullOpinion = !loading && !!opinion && opinion.issues.length > 0;
  const hasDraftOnly = !loading && !hasFullOpinion && hasDraftOpinion;

  return (
    <div className={clsx('space-y-4', className)}>
      <Disclaimer />

      <Card
        header={
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-base font-semibold text-neutral-800 dark:text-white">
              Draft Reasoned Opinion
            </h2>
            {hasFullOpinion && opinion && (
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => exportOpinionAsMarkdown(opinion, comments)}
                  leftIcon={<FileTextIcon className="h-3.5 w-3.5" />}
                >
                  Export Markdown
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => exportOpinionAsText(opinion, comments)}
                  leftIcon={<FileTextIcon className="h-3.5 w-3.5" />}
                >
                  Export Text
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => exportOpinionAsJSON(opinion, comments)}
                  leftIcon={<FileJsonIcon className="h-3.5 w-3.5" />}
                >
                  Export JSON
                </Button>
              </div>
            )}
          </div>
        }
      >
        {loading && (
          <div
            data-testid="reasoning-opinion-placeholder"
            className="flex flex-col items-center justify-center gap-3 py-16 text-center"
          >
            <ScaleIcon className="h-10 w-10 text-neutral-300" aria-hidden="true" />
            <p className="text-sm text-neutral-500">Synthesizing draft analysis…</p>
          </div>
        )}

        {hasDraftOnly && (
          <div
            data-testid="reasoning-opinion-placeholder"
            className="flex flex-col items-center justify-center gap-3 py-16 text-center"
          >
            <ScaleIcon className="h-10 w-10 text-neutral-300" aria-hidden="true" />
            <p className="text-sm text-neutral-500">
              A draft opinion is available. The full review interface is not yet available in
              this build.
            </p>
          </div>
        )}

        {!loading && !hasFullOpinion && !hasDraftOnly && (
          <div
            data-testid="reasoning-opinion-placeholder"
            className="flex flex-col items-center justify-center gap-3 py-16 text-center"
          >
            <ScaleIcon className="h-10 w-10 text-neutral-300" aria-hidden="true" />
            <p className="text-sm font-medium text-neutral-600 dark:text-neutral-300">
              No draft opinion yet
            </p>
            <p className="max-w-sm text-xs text-neutral-400">
              Once reasoning has been generated for this case, the per-issue draft analysis
              with full evidence traceability will render here for review.
            </p>
          </div>
        )}

        {hasFullOpinion && opinion && (
          <div data-testid="reasoning-opinion-content" className="space-y-6">
            {opinion.issues.map((issue) => (
              <IssueSection
                key={issue.issueNodeId}
                issue={issue}
                comments={comments}
                onAddComment={handleAddComment}
                onViewTrace={onViewTrace}
              />
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
