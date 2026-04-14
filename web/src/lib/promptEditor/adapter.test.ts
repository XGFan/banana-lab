import { describe, it, expect } from 'vitest';
import { collectionToPlainPrompt, libraryFromSnippets, snippetsFromLibrary } from './adapter';
import type { PromptCollectionState } from './types';

describe('promptEditor adapter empty library semantics', () => {
  it('keeps empty group as zero prompts when loading from snippet', () => {
    const library = libraryFromSnippets([
      {
        id: 'group-1',
        name: 'Group 1',
        content_fragments: [],
        created_at: '2026-02-09T00:00:00Z',
        updated_at: '2026-02-09T00:00:00Z',
      },
    ]);

    const state = library as unknown as PromptCollectionState;
    expect(state.groupOrder).toEqual(['group-1']);
    expect(state.groups['group-1']?.promptIds).toEqual([]);
    expect(Object.keys(state.prompts)).toEqual([]);
  });

  it('serializes empty group back to empty content_fragments', () => {
    const state: PromptCollectionState = {
      groupOrder: ['group-1'],
      groups: {
        'group-1': {
          id: 'group-1',
          name: 'Group 1',
          promptIds: [],
          collapsed: false,
        },
      },
      prompts: {},
      version: 1,
    };

    const snippets = snippetsFromLibrary(state as any);
    expect(snippets).toHaveLength(1);
    expect(snippets[0]).toMatchObject({
      id: 'group-1',
      name: 'Group 1',
      content_fragments: [],
    });
  });
});

describe('promptEditor adapter multiline semantics', () => {
  it('preserves line breaks when serializing collection to plain prompt', () => {
    const state: PromptCollectionState = {
      groupOrder: ['group-1'],
      groups: {
        'group-1': {
          id: 'group-1',
          name: 'Group 1',
          promptIds: ['p1', 'p2'],
          collapsed: false,
        },
      },
      prompts: {
        p1: {
          id: 'p1',
          content: 'first line\nsecond line',
          type: 'text',
          createdAt: '2026-02-10T00:00:00Z',
          updatedAt: '2026-02-10T00:00:00Z',
        },
        p2: {
          id: 'p2',
          content: 'third line',
          type: 'text',
          createdAt: '2026-02-10T00:00:00Z',
          updatedAt: '2026-02-10T00:00:00Z',
        },
      },
      version: 1,
    };

    expect(collectionToPlainPrompt(state)).toBe('first line\nsecond line\nthird line');
  });

  it('preserves line breaks when serializing library snippets', () => {
    const state: PromptCollectionState = {
      groupOrder: ['group-1'],
      groups: {
        'group-1': {
          id: 'group-1',
          name: 'Group 1',
          promptIds: ['p1', 'p2'],
          collapsed: false,
        },
      },
      prompts: {
        p1: {
          id: 'p1',
          content: 'line A',
          type: 'text',
          createdAt: '2026-02-10T00:00:00Z',
          updatedAt: '2026-02-10T00:00:00Z',
        },
        p2: {
          id: 'p2',
          content: 'line B\nline C',
          type: 'text',
          createdAt: '2026-02-10T00:00:00Z',
          updatedAt: '2026-02-10T00:00:00Z',
        },
      },
      version: 1,
    };

    const snippets = snippetsFromLibrary(state as any);
    expect(snippets).toHaveLength(1);
    expect(snippets[0]?.content_fragments).toEqual([
      { type: 'text', text: 'line A\nline B\nline C' },
    ]);
  });
});
