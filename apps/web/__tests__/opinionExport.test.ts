/**
 * @jest-environment jsdom
 */
import {
  exportOpinionAsJSON,
  exportOpinionAsMarkdown,
  exportOpinionAsText,
  opinionToJSON,
  opinionToMarkdown,
  opinionToText,
} from '@/lib/opinionExport';
import type { CaseOpinion, OpinionComment } from '@/types';

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
        },
      ],
      secondPartyArguments: [],
      evidenceWeights: [
        {
          factNodeId: 'fact-1',
          weight: 0.78,
          kind: 'documentary',
          contradicted: false,
          corroborationCount: 2,
          rationale: 'Signed purchase order corroborated by two arguments.',
        },
      ],
      conclusion: {
        issueNodeId: 'issue-1',
        text: 'The evidence tentatively suggests the contract was validly formed.',
        favoredParty: 'first_party',
        confidence: 0.71,
        weakestLink: 'fact-2 is contradicted.',
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
        },
      ],
    },
  ],
};

const COMMENTS: OpinionComment[] = [
  {
    id: 'comment-1',
    issueNodeId: 'issue-1',
    text: 'Confirm the signature date before sign-off.',
    author: 'Jane Judge',
    occurredAt: '2026-01-06T00:00:00Z',
  },
];

describe('opinionToMarkdown', () => {
  it('includes the disclaimer, issue text, arguments, evidence weights, uncertainty, and comments', () => {
    const markdown = opinionToMarkdown(OPINION, COMMENTS);
    expect(markdown).toContain('Non-Binding Draft Analysis');
    expect(markdown).toContain('Was the contract validly formed?');
    expect(markdown).toContain('The signed purchase orders establish offer and acceptance.');
    expect(markdown).toContain('Signed purchase order corroborated by two arguments.');
    expect(markdown).toContain('fact-2 is contradicted by the opposing party');
    expect(markdown).toContain('Confirm the signature date before sign-off.');
    expect(markdown).toContain('Jane Judge');
  });
});

describe('opinionToText', () => {
  it('strips Markdown emphasis syntax while keeping the content', () => {
    const text = opinionToText(OPINION, COMMENTS);
    expect(text).not.toContain('#');
    expect(text).not.toContain('**');
    expect(text).toContain('Was the contract validly formed?');
    expect(text).toContain('Non-Binding Draft Analysis');
  });
});

describe('opinionToJSON', () => {
  it('produces valid JSON with the disclaimer and comments attached', () => {
    const json = opinionToJSON(OPINION, COMMENTS);
    const parsed = JSON.parse(json);
    expect(parsed.caseId).toBe('case-1');
    expect(parsed.disclaimer).toMatch(/non-binding/i);
    expect(parsed.issues).toHaveLength(1);
    expect(parsed.comments).toHaveLength(1);
    expect(parsed.comments[0].author).toBe('Jane Judge');
  });
});

describe('export* download triggers', () => {
  let createObjectURL: jest.Mock;
  let revokeObjectURL: jest.Mock;
  let clickSpy: jest.SpyInstance;

  beforeEach(() => {
    createObjectURL = jest.fn(() => 'blob:mock-url');
    revokeObjectURL = jest.fn();
    URL.createObjectURL = createObjectURL;
    URL.revokeObjectURL = revokeObjectURL;
    clickSpy = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  });

  afterEach(() => {
    clickSpy.mockRestore();
  });

  it('exportOpinionAsMarkdown names the file after the case id with a .md extension', () => {
    let capturedDownloadName: string | null = null;
    clickSpy.mockImplementation(function (this: HTMLAnchorElement) {
      capturedDownloadName = this.download;
    });
    exportOpinionAsMarkdown(OPINION, COMMENTS);
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(capturedDownloadName).toBe('draft-opinion-case-1.md');
  });

  it('exportOpinionAsText names the file with a .txt extension', () => {
    let capturedDownloadName: string | null = null;
    clickSpy.mockImplementation(function (this: HTMLAnchorElement) {
      capturedDownloadName = this.download;
    });
    exportOpinionAsText(OPINION, COMMENTS);
    expect(capturedDownloadName).toBe('draft-opinion-case-1.txt');
  });

  it('exportOpinionAsJSON names the file with a .json extension and revokes the URL', () => {
    let capturedDownloadName: string | null = null;
    clickSpy.mockImplementation(function (this: HTMLAnchorElement) {
      capturedDownloadName = this.download;
    });
    exportOpinionAsJSON(OPINION, COMMENTS);
    expect(capturedDownloadName).toBe('draft-opinion-case-1.json');
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:mock-url');
  });
});
