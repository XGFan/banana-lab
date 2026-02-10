import type { PromptFragment, PromptSnippet } from '../api';
import type { PromptCollectionState, PromptItem } from './types';
import type { PromptEditorValue, PromptEditorLibrary } from 'prompt-editor-sdk';

export type PromptEditorInitState = {
  value: PromptEditorValue;
  library: PromptEditorLibrary;
};

const generateId = () => Math.random().toString(36).substring(2, 9);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function toCollectionState(value: unknown): PromptCollectionState | null {
  if (!isRecord(value)) return null;
  if (!Array.isArray(value.groupOrder) || !isRecord(value.groups) || !isRecord(value.prompts)) {
    return null;
  }

  const now = new Date().toISOString();
  const prompts: Record<string, PromptItem> = {};

  for (const [key, rawPrompt] of Object.entries(value.prompts)) {
    if (!isRecord(rawPrompt)) continue;
    const id = typeof rawPrompt.id === 'string' && rawPrompt.id.trim() ? rawPrompt.id : key;
    const content = typeof rawPrompt.content === 'string' ? rawPrompt.content : '';
    const createdAt = typeof rawPrompt.createdAt === 'string' && rawPrompt.createdAt.trim()
      ? rawPrompt.createdAt
      : now;
    const updatedAt = typeof rawPrompt.updatedAt === 'string' && rawPrompt.updatedAt.trim()
      ? rawPrompt.updatedAt
      : now;
    prompts[id] = {
      ...(rawPrompt as Record<string, unknown>),
      id,
      content,
      createdAt,
      updatedAt,
    } as PromptItem;
  }

  const groups: PromptCollectionState['groups'] = {};
  const groupOrder = (value.groupOrder as unknown[])
    .filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    .filter((id) => isRecord((value.groups as Record<string, unknown>)[id]))
    .map((id) => {
      const rawGroup = (value.groups as Record<string, unknown>)[id] as Record<string, unknown>;
      const promptIds = Array.isArray(rawGroup.promptIds)
        ? (rawGroup.promptIds as unknown[])
            .filter((pid): pid is string => typeof pid === 'string' && pid.trim().length > 0)
            .filter((pid) => Boolean(prompts[pid]))
        : [];
      groups[id] = {
        ...(rawGroup as Record<string, unknown>),
        id: typeof rawGroup.id === 'string' && rawGroup.id.trim() ? rawGroup.id : id,
        name: typeof rawGroup.name === 'string' && rawGroup.name.trim() ? rawGroup.name : 'Default',
        promptIds,
        collapsed: typeof rawGroup.collapsed === 'boolean' ? rawGroup.collapsed : false,
      };
      return id;
    });

  return {
    ...(value as Record<string, unknown>),
    groupOrder,
    groups,
    prompts,
    version: typeof value.version === 'number' ? value.version : 1,
  } as PromptCollectionState;
}

function fragmentsToText(contentFragments: unknown): string {
  if (!Array.isArray(contentFragments)) {
    return '';
  }
  return contentFragments
    .map((f) => (isRecord(f) && typeof f.text === 'string' ? f.text : ''))
    .filter((text) => text.trim().length > 0)
    .join(', ');
}

export function emptyCollectionState(): PromptCollectionState {
  return {
    groupOrder: [],
    groups: {},
    prompts: {},
    version: 1,
  };
}

export function collectionFromPlainPrompt(prompt: string): PromptCollectionState {
  if (!prompt.trim()) {
    return emptyCollectionState();
  }

  const groupId = generateId();
  const promptId = generateId();
  const now = new Date().toISOString();

  return {
    groupOrder: [groupId],
    groups: {
      [groupId]: {
        id: groupId,
        name: 'Default',
        promptIds: [promptId],
        collapsed: false,
      },
    },
    prompts: {
      [promptId]: {
        id: promptId,
        content: prompt,
        type: 'text',
        createdAt: now,
        updatedAt: now,
      },
    },
    version: 1,
  };
}

export function collectionToPlainPrompt(state: PromptCollectionState): string {
  if (!state.groupOrder || state.groupOrder.length === 0) {
    return '';
  }

  const parts: string[] = [];

  state.groupOrder.forEach((groupId) => {
    const group = state.groups[groupId];
    if (group && group.promptIds) {
      group.promptIds.forEach((promptId) => {
        const prompt = state.prompts[promptId];
        if (prompt && prompt.content) {
          parts.push(prompt.content);
        }
      });
    }
  });

  return parts.join(', ');
}

export function normalizeLegacyFragmentsToCollection(fragments: PromptFragment[]): PromptCollectionState {
  if (!fragments || fragments.length === 0) {
    return emptyCollectionState();
  }

  const groupId = generateId();
  const promptIds: string[] = [];
  const prompts: Record<string, PromptItem> = {};
  const now = new Date().toISOString();

  fragments.forEach((fragment) => {
    const promptId = (fragment as any).id || generateId();
    promptIds.push(promptId);
    prompts[promptId] = {
      ...fragment, 
      id: promptId,
      content: fragment.text || '',
      type: fragment.type || 'text',
      createdAt: now,
      updatedAt: now,
    };
  });

  return {
    groupOrder: [groupId],
    groups: {
      [groupId]: {
        id: groupId,
        name: 'Default',
        promptIds,
        collapsed: false,
      },
    },
    prompts,
    version: 1,
  };
}

export function libraryFromSnippets(snippets: PromptSnippet[]): PromptEditorLibrary {
  const state: PromptCollectionState = {
    groupOrder: [],
    groups: {},
    prompts: {},
    version: 1,
  };

  snippets.forEach((snippet) => {
    const groupId = snippet.id;
    const promptId = `${snippet.id}::p0`;
    const content = fragmentsToText(snippet.content_fragments);
    const now = snippet.updated_at || new Date().toISOString();
    const hasContent = content.trim().length > 0;

    state.groupOrder.push(groupId);
    state.groups[groupId] = {
      id: groupId,
      name: snippet.name,
      promptIds: hasContent ? [promptId] : [],
      collapsed: false,
    };

    if (hasContent) {
      state.prompts[promptId] = {
        id: promptId,
        content: content,
        type: 'text',
        createdAt: snippet.created_at || now,
        updatedAt: snippet.updated_at || now,
      };
    }
  });

  return state as unknown as PromptEditorLibrary;
}

export function snippetsFromLibrary(library: PromptEditorLibrary): PromptSnippet[] {
  const state = toCollectionState(library);
  if (!state) return [];

  return state.groupOrder.map((groupId) => {
    const group = state.groups[groupId];
    if (!group) return null;

    const contentParts: string[] = [];
    if (group.promptIds) {
      group.promptIds.forEach((pid) => {
        const p = state.prompts[pid];
        if (p) contentParts.push(p.content);
      });
    }
    
    const joinedContent = contentParts.join(', ');
    const hasContent = joinedContent.trim().length > 0;

    const snippet: PromptSnippet = {
      id: group.id,
      name: group.name,
      content_fragments: hasContent ? [{ type: 'text', text: joinedContent }] : [],
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    return snippet;
  }).filter((s): s is PromptSnippet => s !== null);
}

export function toSdkValue(
  currentPrompt: string,
  currentFragments: unknown,
  librarySnippets: PromptSnippet[]
): PromptEditorInitState {
  let fragments: PromptCollectionState;

  const normalized = toCollectionState(currentFragments);

  if (normalized) {
    fragments = normalized;
  } else if (Array.isArray(currentFragments) && currentFragments.length > 0) {
    fragments = normalizeLegacyFragmentsToCollection(currentFragments as PromptFragment[]);
  } else {
    fragments = collectionFromPlainPrompt(currentPrompt);
  }

  return {
    value: {
      fragments,
    },
    library: libraryFromSnippets(librarySnippets),
  };
}
