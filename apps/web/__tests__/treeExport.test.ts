/**
 * @jest-environment jsdom
 */
import { exportTreeAsJSON, exportTreeAsSVG } from '@/lib/treeExport';
import type { ReasoningTree } from '@/types';

const TREE: ReasoningTree = {
  caseId: 'case-1',
  nodes: [
    {
      id: 'issue-1',
      type: 'issue',
      caseId: 'case-1',
      text: 'Was the contract validly formed?',
      confidence: 0.9,
      createdAt: '2026-01-01T00:00:00Z',
    },
  ],
  edges: [],
};

describe('exportTreeAsJSON', () => {
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

  it('creates a blob URL, clicks a download anchor, and revokes the URL', () => {
    exportTreeAsJSON(TREE, 'case-1');
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:mock-url');
  });

  it('names the file after the case id', () => {
    let capturedDownloadName: string | null = null;
    clickSpy.mockImplementation(function (this: HTMLAnchorElement) {
      capturedDownloadName = this.download;
    });
    exportTreeAsJSON(TREE, 'case-42');
    expect(capturedDownloadName).toBe('reasoning-tree-case-42.json');
  });
});

describe('exportTreeAsSVG', () => {
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

  it('serializes the given svg element and triggers a download', () => {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    document.body.appendChild(svg);
    exportTreeAsSVG(svg, 'case-1');
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(clickSpy).toHaveBeenCalledTimes(1);
  });
});
