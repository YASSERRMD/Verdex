import type { CaseOpinion, IssueOpinion, OpinionArgument, OpinionComment } from '@/types';

/**
 * Triggers a browser download of `content` as a file named `filename` with
 * the given MIME `type`. Mirrors treeExport.ts's `triggerDownload` exactly
 * (same jsdom-safety rationale: callers can stub `URL.createObjectURL` in
 * tests to assert an export was attempted without a real browser download).
 */
function triggerDownload(content: string, filename: string, type: string): void {
  const blob = new Blob([content], { type });
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

const PARTY_LABELS: Record<OpinionArgument['partyId'], string> = {
  first_party: 'First Party',
  second_party: 'Second Party',
};

function formatArgument(argument: OpinionArgument, indent = '    '): string {
  const lines = [
    `${indent}- [${PARTY_LABELS[argument.partyId]}] ${argument.claim} (strength ${Math.round(argument.strength * 100)}%)`,
  ];
  if (argument.counterarguments && argument.counterarguments.length > 0) {
    lines.push(`${indent}  Anticipated counterarguments: ${argument.counterarguments.join('; ')}`);
  }
  return lines.join('\n');
}

function formatIssueMarkdown(issue: IssueOpinion, comments: OpinionComment[]): string {
  const parts: string[] = [`## ${issue.issueText}`, ''];

  parts.push('**Draft analysis (non-binding):** ' + issue.conclusion.text, '');
  if (issue.conclusion.weakestLink) {
    parts.push(`**Weakest link:** ${issue.conclusion.weakestLink}`, '');
  }

  parts.push('### First Party Arguments', '');
  if (issue.firstPartyArguments.length === 0) {
    parts.push('_None recorded._', '');
  } else {
    issue.firstPartyArguments.forEach((a) => parts.push(formatArgument(a, '- ')));
    parts.push('');
  }

  parts.push('### Second Party Arguments', '');
  if (issue.secondPartyArguments.length === 0) {
    parts.push('_None recorded._', '');
  } else {
    issue.secondPartyArguments.forEach((a) => parts.push(formatArgument(a, '- ')));
    parts.push('');
  }

  if (issue.evidenceWeights.length > 0) {
    parts.push('### Evidence Weights', '');
    issue.evidenceWeights.forEach((w) => {
      parts.push(
        `- Fact ${w.factNodeId}: weight ${Math.round(w.weight * 100)}%${w.contradicted ? ' (contradicted)' : ''} — ${w.rationale}`,
      );
    });
    parts.push('');
  }

  if (issue.uncertainties.length > 0) {
    parts.push('### Uncertainty Callouts', '');
    issue.uncertainties.forEach((u) => {
      parts.push(`- [${u.source}] ${u.caveat}`);
    });
    parts.push('');
  }

  const issueComments = comments.filter((c) => c.issueNodeId === issue.issueNodeId);
  if (issueComments.length > 0) {
    parts.push('### Judge Comments', '');
    issueComments.forEach((c) => {
      parts.push(`- ${c.author} (${new Date(c.occurredAt).toLocaleString()}): ${c.text}`);
    });
    parts.push('');
  }

  return parts.join('\n');
}

const DISCLAIMER_TEXT =
  'This system produces non-binding draft analyses only. All outputs require review and ' +
  'sign-off by a qualified judge before any legal use or publication.';

/**
 * Renders a CaseOpinion (plus any client-side judge comments) as a
 * standalone Markdown document: disclaimer first, then one section per
 * issue with both parties' arguments, evidence weights, uncertainty
 * callouts, and comments.
 */
export function opinionToMarkdown(opinion: CaseOpinion, comments: OpinionComment[]): string {
  const parts = [
    `# Draft Reasoned Opinion — Case ${opinion.caseId}`,
    '',
    `> **Non-Binding Draft Analysis.** ${DISCLAIMER_TEXT}`,
    '',
    `Generated ${new Date(opinion.generatedAt).toLocaleString()}`,
    '',
  ];
  opinion.issues.forEach((issue) => parts.push(formatIssueMarkdown(issue, comments)));
  return parts.join('\n');
}

/**
 * Renders a CaseOpinion (plus comments) as plain text — the same content
 * as opinionToMarkdown, without Markdown emphasis syntax, for reviewers
 * who want a plain-text copy.
 */
export function opinionToText(opinion: CaseOpinion, comments: OpinionComment[]): string {
  return opinionToMarkdown(opinion, comments)
    .replace(/^#+\s*/gm, '')
    .replace(/\*\*/g, '')
    .replace(/^>\s*/gm, '');
}

/** Renders a CaseOpinion (plus comments) as pretty-printed structured JSON. */
export function opinionToJSON(opinion: CaseOpinion, comments: OpinionComment[]): string {
  return JSON.stringify(
    {
      disclaimer: DISCLAIMER_TEXT,
      ...opinion,
      comments,
    },
    null,
    2,
  );
}

export function exportOpinionAsMarkdown(opinion: CaseOpinion, comments: OpinionComment[]): void {
  triggerDownload(opinionToMarkdown(opinion, comments), `draft-opinion-${opinion.caseId}.md`, 'text/markdown');
}

export function exportOpinionAsText(opinion: CaseOpinion, comments: OpinionComment[]): void {
  triggerDownload(opinionToText(opinion, comments), `draft-opinion-${opinion.caseId}.txt`, 'text/plain');
}

export function exportOpinionAsJSON(opinion: CaseOpinion, comments: OpinionComment[]): void {
  triggerDownload(opinionToJSON(opinion, comments), `draft-opinion-${opinion.caseId}.json`, 'application/json');
}
