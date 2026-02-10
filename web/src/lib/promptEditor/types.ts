export interface PromptItem {
  id: string;
  content: string;
  type?: string;
  createdAt: string;
  updatedAt: string;
  [key: string]: unknown;
}

export interface PromptGroup {
  id: string;
  name: string;
  promptIds: string[];
  collapsed: boolean;
  [key: string]: unknown;
}

export interface PromptCollectionState {
  version?: number;
  groupOrder: string[];
  groups: Record<string, PromptGroup>;
  prompts: Record<string, PromptItem>;
  meta?: Record<string, unknown>;
  [key: string]: unknown;
}
