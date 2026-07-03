/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ReasoningOpinionPanel } from '@/components/workspace/ReasoningOpinionPanel';
import type { CaseOpinion } from '@/types';

// Real banned verdict/directive words from packages/irac/guardrail.go's
// verdictLanguageWordlist, the same list packages/guardrail.CheckText
// (verdict.go) gates all reasoning-output text against. Mirroring the
// actual wordlist here (rather than a made-up generic list) means this
// test asserts the same guardrail the backend enforces, not just an
// approximation of it.
const VERDICT_WORDLIST = [
  'guilty',
  'liable',
  'shall pay',
  'is ordered',
  'is hereby ordered',
  'judgment for',
  'convicted',
  'acquitted',
  'sentenced',
];

const OPINION: CaseOpinion = {
  caseId: 'case-1',
  generatedAt: '2026-01-05T00:00:00Z',
  issues: [
    {
      issueNodeId: 'issue-1',
      issueText: 'Was the contract validly formed?',
      firstPartyArguments: [
        {
          id: 'arg-fp-1',
          issueNodeId: 'issue-1',
          partyId: 'first_party',
          claim: 'The signed purchase orders establish offer and acceptance.',
          supportingFactIds: ['fact-1'],
          supportingRuleIds: ['rule-1'],
          strength: 0.82,
          grounded: true,
          counterarguments: ['The purchase order was unsigned by the second party.'],
        },
      ],
      secondPartyArguments: [
        {
          id: 'arg-sp-1',
          issueNodeId: 'issue-1',
          partyId: 'second_party',
          claim: 'No consideration was exchanged, so no contract was formed.',
          supportingFactIds: ['fact-2'],
          supportingRuleIds: [],
          strength: 0.4,
          grounded: true,
        },
      ],
      evidenceWeights: [
        {
          factNodeId: 'fact-1',
          weight: 0.78,
          kind: 'documentary',
          contradicted: false,
          corroborationCount: 2,
          rationale: 'Signed purchase order corroborated by two arguments.',
        },
        {
          factNodeId: 'fact-2',
          weight: 0.3,
          kind: 'testimony',
          contradicted: true,
          corroborationCount: 1,
          rationale: 'Witness testimony contradicted by the signed purchase order.',
        },
      ],
      conclusion: {
        issueNodeId: 'issue-1',
        text: 'The evidence and applicable rule tentatively suggest the contract was validly formed.',
        favoredParty: 'first_party',
        confidence: 0.71,
        weakestLink: 'fact-2 is contradicted and carries a low evidentiary weight.',
        supportingFactIds: ['fact-1'],
        supportingRuleIds: ['rule-1'],
        grounded: true,
      },
      uncertainties: [
        {
          issueNodeId: 'issue-1',
          source: 'evidence',
          severity: 0.6,
          impactRank: 1,
          impactScore: 0.55,
          caveat: 'fact-2 is contradicted by the opposing party’s cited evidence.',
          detail: 'fact-2',
        },
      ],
    },
    {
      issueNodeId: 'issue-2',
      issueText: 'Was notice properly served?',
      firstPartyArguments: [],
      secondPartyArguments: [
        {
          id: 'arg-sp-2',
          issueNodeId: 'issue-2',
          partyId: 'second_party',
          claim: 'Notice was served by certified mail per the governing rule.',
          supportingFactIds: ['fact-3'],
          supportingRuleIds: ['rule-2'],
          strength: 0.9,
          grounded: true,
        },
      ],
      evidenceWeights: [],
      conclusion: {
        issueNodeId: 'issue-2',
        text: 'Notice appears to have been properly served on the current record.',
        favoredParty: 'second_party',
        confidence: 0.88,
        supportingFactIds: ['fact-3'],
        supportingRuleIds: ['rule-2'],
        grounded: true,
      },
      // No uncertainties for this issue — callouts must not appear.
      uncertainties: [],
    },
  ],
};

describe('ReasoningOpinionPanel', () => {
  it('always renders the non-binding disclaimer', () => {
    render(<ReasoningOpinionPanel />);
    expect(screen.getByLabelText(/non-binding disclaimer/i)).toBeInTheDocument();
    expect(screen.getByText(/non-binding draft analysis/i)).toBeInTheDocument();
  });

  it('renders the disclaimer prominently at the top, before any draft content', () => {
    const { container } = render(<ReasoningOpinionPanel opinion={OPINION} />);
    const disclaimer = screen.getByLabelText(/non-binding disclaimer/i);
    const heading = screen.getByText('Draft Reasoned Opinion');
    // The disclaimer node must precede the panel heading in document order.
    const position = disclaimer.compareDocumentPosition(heading);
    // eslint-disable-next-line no-bitwise
    expect(position & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(container.firstElementChild).toBe(disclaimer.parentElement);
  });

  it('renders an empty state when there is no draft opinion', () => {
    render(<ReasoningOpinionPanel />);
    expect(screen.getByText(/no draft opinion yet/i)).toBeInTheDocument();
  });

  it('renders a loading state while synthesizing', () => {
    render(<ReasoningOpinionPanel loading />);
    expect(screen.getByText(/synthesizing draft analysis/i)).toBeInTheDocument();
  });

  it('renders a draft-available message once a draft opinion exists without full content', () => {
    render(<ReasoningOpinionPanel hasDraftOpinion />);
    expect(screen.getByText(/a draft opinion is available/i)).toBeInTheDocument();
  });

  it('never uses verdict language in the empty/loading placeholder', () => {
    render(<ReasoningOpinionPanel hasDraftOpinion />);
    expect(screen.getByTestId('reasoning-opinion-placeholder')).not.toHaveTextContent(
      /verdict|ruling|final decision/i,
    );
  });

  describe('with a full opinion', () => {
    it('renders one section per issue with the issue framing and its tentative conclusion', () => {
      render(<ReasoningOpinionPanel opinion={OPINION} />);
      for (const issue of OPINION.issues) {
        const section = screen.getByTestId(`opinion-issue-${issue.issueNodeId}`);
        expect(within(section).getByText(issue.issueText)).toBeInTheDocument();
        const conclusion = screen.getByTestId(`opinion-conclusion-${issue.issueNodeId}`);
        expect(within(conclusion).getByText(issue.conclusion.text)).toBeInTheDocument();
      }
    });

    it('shows both parties’ arguments side by side for each issue', () => {
      render(<ReasoningOpinionPanel opinion={OPINION} />);
      const firstPartyCol = screen.getByTestId('opinion-first-party-args-issue-1');
      const secondPartyCol = screen.getByTestId('opinion-second-party-args-issue-1');
      expect(within(firstPartyCol).getByText(/signed purchase orders establish/i)).toBeInTheDocument();
      expect(within(secondPartyCol).getByText(/no consideration was exchanged/i)).toBeInTheDocument();

      // Issue 2 has no first-party arguments — the column still renders
      // with an explicit "none" message rather than disappearing, so the
      // side-by-side layout is stable across issues.
      const emptyFirstPartyCol = screen.getByTestId('opinion-first-party-args-issue-2');
      expect(within(emptyFirstPartyCol).getByText(/no arguments recorded/i)).toBeInTheDocument();
    });

    it('displays evidence weights inline, including contradiction and corroboration', () => {
      render(<ReasoningOpinionPanel opinion={OPINION} />);
      const weights = screen.getByTestId('opinion-evidence-weights-issue-1');
      expect(within(weights).getByText(/signed purchase order corroborated/i)).toBeInTheDocument();
      expect(within(weights).getByText(/weight 78%/i)).toBeInTheDocument();
      expect(within(weights).getAllByText(/contradicted/i).length).toBeGreaterThan(0);
      expect(within(weights).getByText(/2 corroborating/i)).toBeInTheDocument();
    });

    it('shows uncertainty callouts only for issues with flagged uncertainty', () => {
      render(<ReasoningOpinionPanel opinion={OPINION} />);
      const issue1Callouts = screen.getByTestId('opinion-uncertainty-issue-1');
      expect(within(issue1Callouts).getByText(/contradicted by the opposing party/i)).toBeInTheDocument();

      // issue-2 has no uncertainties — no callout block should render at all.
      expect(screen.queryByTestId('opinion-uncertainty-issue-2')).not.toBeInTheDocument();
    });

    it('surfaces the weakest-link caveat on the conclusion when present', () => {
      render(<ReasoningOpinionPanel opinion={OPINION} />);
      const conclusion = screen.getByTestId('opinion-conclusion-issue-1');
      expect(within(conclusion).getByText(/weakest link/i)).toBeInTheDocument();
      expect(within(conclusion).getByText(/contradicted and carries a low evidentiary weight/i)).toBeInTheDocument();

      // issue-2's conclusion has no weakestLink — the callout must be absent.
      const conclusion2 = screen.getByTestId('opinion-conclusion-issue-2');
      expect(within(conclusion2).queryByText(/weakest link/i)).not.toBeInTheDocument();
    });

    it('invokes onViewTrace with the issue and node IDs when a trace link is clicked', async () => {
      const user = userEvent.setup();
      const onViewTrace = jest.fn();
      render(<ReasoningOpinionPanel opinion={OPINION} onViewTrace={onViewTrace} />);

      const conclusion = screen.getByTestId('opinion-conclusion-issue-1');
      await user.click(within(conclusion).getByRole('button', { name: /view full trace/i }));

      expect(onViewTrace).toHaveBeenCalledWith('issue-1', 'fact-1');
    });

    it('invokes onViewTrace from a per-argument "view supporting nodes" link', async () => {
      const user = userEvent.setup();
      const onViewTrace = jest.fn();
      render(<ReasoningOpinionPanel opinion={OPINION} onViewTrace={onViewTrace} />);

      const argument = screen.getByTestId('opinion-argument-arg-sp-2');
      await user.click(within(argument).getByRole('button', { name: /view supporting nodes/i }));

      expect(onViewTrace).toHaveBeenCalledWith('issue-2', 'fact-3');
    });

    it('does not render trace-link buttons when onViewTrace is not provided', () => {
      render(<ReasoningOpinionPanel opinion={OPINION} />);
      expect(screen.queryByRole('button', { name: /view full trace/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /view supporting nodes/i })).not.toBeInTheDocument();
    });

    it('lets a judge add a comment to an issue, persisted in local state with author and timestamp', async () => {
      const user = userEvent.setup();
      render(<ReasoningOpinionPanel opinion={OPINION} />);

      const commentBox = screen.getByLabelText(/add a comment for was the contract validly formed/i);
      await user.type(commentBox, 'Confirm the purchase order signature date before sign-off.');
      await user.click(within(screen.getByTestId('opinion-comments-issue-1')).getByRole('button', { name: /add/i }));

      const commentsSection = screen.getByTestId('opinion-comments-issue-1');
      expect(
        within(commentsSection).getByText(/confirm the purchase order signature date/i),
      ).toBeInTheDocument();
      expect(within(commentsSection).getByText(/current reviewer/i)).toBeInTheDocument();

      // The textarea clears after a successful add.
      expect(commentBox).toHaveValue('');

      // A comment added under issue-1 must not leak into issue-2's list.
      expect(
        within(screen.getByTestId('opinion-comments-issue-2')).getByText(/no comments yet/i),
      ).toBeInTheDocument();
    });

    it('does not add a comment when the draft is empty or whitespace', async () => {
      const user = userEvent.setup();
      render(<ReasoningOpinionPanel opinion={OPINION} />);

      const commentsSection = screen.getByTestId('opinion-comments-issue-1');
      const addButton = within(commentsSection).getByRole('button', { name: /add/i });
      expect(addButton).toBeDisabled();

      const commentBox = screen.getByLabelText(/add a comment for was the contract validly formed/i);
      await user.type(commentBox, '   ');
      expect(addButton).toBeDisabled();
    });

    it('renders export controls that produce Markdown, text, and JSON output', async () => {
      const user = userEvent.setup();
      const createObjectURL = jest.fn(() => 'blob:mock');
      const revokeObjectURL = jest.fn();
      URL.createObjectURL = createObjectURL;
      URL.revokeObjectURL = revokeObjectURL;
      let capturedParts: BlobPart[] = [];
      const originalBlob = global.Blob;
      // Capture the content parts passed to the Blob constructor so the
      // test can assert on the exported content itself, not just that a
      // download was attempted (jsdom's Blob does not reliably support
      // Blob.text() in this environment, so read the constructor args
      // directly instead).
      // @ts-expect-error -- test double intentionally narrows the ctor
      global.Blob = jest.fn((parts: BlobPart[], options?: BlobPropertyBag) => {
        capturedParts = parts;
        return new originalBlob(parts, options);
      });
      const clickSpy = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

      render(<ReasoningOpinionPanel opinion={OPINION} />);

      await user.click(screen.getByRole('button', { name: /export markdown/i }));
      expect(clickSpy).toHaveBeenCalledTimes(1);
      const markdownText = String(capturedParts[0]);
      expect(markdownText).toContain('Non-Binding Draft Analysis');
      expect(markdownText).toContain('Was the contract validly formed?');
      expect(markdownText).toContain('First Party Arguments');

      await user.click(screen.getByRole('button', { name: /export text/i }));
      const textContent = String(capturedParts[0]);
      expect(textContent).toContain('Was the contract validly formed?');
      expect(textContent).not.toContain('##');

      await user.click(screen.getByRole('button', { name: /export json/i }));
      const jsonContent = String(capturedParts[0]);
      const parsed = JSON.parse(jsonContent);
      expect(parsed.caseId).toBe('case-1');
      expect(parsed.issues).toHaveLength(2);
      expect(parsed.disclaimer).toMatch(/non-binding/i);

      global.Blob = originalBlob;
      clickSpy.mockRestore();
    });

    it('never renders verdict or directive language anywhere in the full opinion content', () => {
      render(<ReasoningOpinionPanel opinion={OPINION} />);
      const content = screen.getByTestId('reasoning-opinion-content');
      const renderedText = content.textContent?.toLowerCase() ?? '';
      for (const word of VERDICT_WORDLIST) {
        expect(renderedText).not.toContain(word.toLowerCase());
      }
    });

    it('does not render export controls or issue sections in the empty state', () => {
      render(<ReasoningOpinionPanel />);
      expect(screen.queryByRole('button', { name: /export markdown/i })).not.toBeInTheDocument();
      expect(screen.queryByTestId('reasoning-opinion-content')).not.toBeInTheDocument();
    });
  });
});
