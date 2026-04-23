export type PromptMode = 'txt2img' | 'img2img';

export interface ModelInfo {
  id: string;
  label: string;
  allowed_sizes: string[];
  allowed_aspect_ratios: string[];
  default_size: string;
}

export interface PromptFragment {
  type: string;
  text?: string;
  [key: string]: unknown;
}

export type PromptCollectionState = {
  groupOrder: string[];
  groups: Record<
    string,
    {
      id: string;
      name: string;
      promptIds: string[];
      collapsed: boolean;
    }
  >;
  prompts: Record<
    string,
    {
      id: string;
      content: string;
      createdAt: string;
      updatedAt: string;
    }
  >;
};

export interface PromptNode<TFragments = PromptCollectionState> {
  id: string;
  parent_id?: string;
  mode: PromptMode;
  prompt: string;
  fragments?: TFragments;
  title: string;
  pinned: boolean;
  sort_order?: number;
  last_used_at: string;
  archived_at?: string;
  negative_prompt?: string;
  cfg_scale?: number;
  aspect_ratio?: string;
  image_size?: string;
  model_id?: string;
  seed?: number;
  steps?: number;
  reference_image_ids?: string[];
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface Image {
  id: string;
  node_id?: string;
  origin: 'generated' | 'uploaded';
  file_path: string;
  file_size?: number;
  width?: number;
  height?: number;
  created_at: string;
}

export interface Asset {
  id: string;
  domain: 'txt2img' | 'img2img';
  origin: 'generated' | 'uploaded';
  file_path: string;
  file_size?: number;
  width?: number;
  height?: number;
  created_at: string;
  updated_at: string;
}

export interface Result {
  id: string;
  node_id: string;
  asset_id: string;
  status: 'active' | 'archived' | 'deleted';
  source_asset_id?: string;
  source_domain?: string;
  archived_at?: string;
  deleted_at?: string;
  created_at: string;
  updated_at: string;
}

export interface GenerateRequest {
  mode: PromptMode;
  parent_node_id?: string;
  prompt: string;
  fragments?: PromptCollectionState;
  negative_prompt?: string;
  cfg_scale?: number;
  aspect_ratio?: string;
  image_size?: string;
  seed?: number;
  steps?: number;
  reference_image_ids: string[];
  width: number;
  height: number;
  model: string;
}

export interface GenerateResponse {
  node: PromptNode<PromptFragment[]>;
  images: Image[];
  results?: Result[];
}

export interface PromptTreeResponse {
  root_id: string;
  nodes: PromptNode[];
}

export interface PromptSnippet {
  id: string;
  name: string;
  content_fragments: PromptFragment[];
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface CreatePromptSnippetRequest {
  id?: string;
  name: string;
  content_fragments: PromptFragment[];
}

export interface UpdatePromptSnippetRequest {
  name?: string;
  content_fragments?: PromptFragment[];
}

export interface ApiError extends Error {
  status?: number;
  code?: string;
}

export interface ArchiveItem {
  id: string;
  domain: 'txt2img' | 'img2img';
  archived_at: string;
  asset: {
    file_path: string;
  };
}

export interface ArchiveResponse {
  txt2img: ArchiveItem[];
  img2img: ArchiveItem[];
}

async function apiFetch<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);

  if (!headers.has('Content-Type') && !(init?.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(input, {
    ...init,
    headers,
  });

  if (!response.ok) {
    let errorMessage = `API Error: ${response.status} ${response.statusText}`;
    let code: string | undefined;
    try {
      const errorData = await response.json();
      if (errorData && typeof errorData === 'object') {
        if ('error' in errorData && typeof errorData.error === 'string') {
          code = errorData.error;
          errorMessage = `${errorMessage} - ${errorData.error}`;
        } else if ('message' in errorData && errorData.message) {
          errorMessage = `${errorMessage} - ${errorData.message}`;
        }
      }
    } catch {
      // intentionally ignored
    }
    const apiError = new Error(errorMessage) as ApiError;
    apiError.status = response.status;
    apiError.code = code;
    throw apiError;
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

export const api = {
  async listPrompts(params?: {
    mode?: PromptMode;
    limit?: number;
    offset?: number;
    include_archived?: boolean;
  }): Promise<PromptNode[]> {
    const searchParams = new URLSearchParams();
    if (params?.mode) searchParams.set('mode', params.mode);
    if (params?.limit !== undefined) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.include_archived !== undefined) searchParams.set('include_archived', String(params.include_archived));

    const queryString = searchParams.toString();
    const url = `/api/prompts${queryString ? `?${queryString}` : ''}`;
    return apiFetch<PromptNode[]>(url);
  },

  async updatePrompt(
    id: string,
    patch: Partial<Pick<PromptNode, 'title' | 'pinned' | 'sort_order'>> & { archived?: boolean }
  ): Promise<PromptNode> {
    return apiFetch<PromptNode>(`/api/prompts/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    });
  },

  async generate(req: GenerateRequest): Promise<GenerateResponse> {
    return apiFetch<GenerateResponse>('/api/generate', {
      method: 'POST',
      body: JSON.stringify(req),
    });
  },

  async upload(file: File): Promise<{ image: Image }> {
    const formData = new FormData();
    formData.append('file', file);

    return apiFetch<{ image: Image }>('/api/upload', {
      method: 'POST',
      body: formData,
    });
  },

  async listLibrary(params: {
    page?: number;
    limit?: number;
    type?: 'txt2img' | 'img2img';
    date?: 'today' | 'yesterday' | 'older';
  }): Promise<Image[]> {
    const searchParams = new URLSearchParams();
    if (params.page !== undefined) searchParams.set('page', params.page.toString());
    if (params.limit !== undefined) searchParams.set('limit', params.limit.toString());
    if (params.type) searchParams.set('type', params.type);
    if (params.date) searchParams.set('date', params.date);

    const queryString = searchParams.toString();
    const url = `/api/library${queryString ? `?${queryString}` : ''}`;
    return apiFetch<Image[]>(url);
  },

  async getArchive(params?: { from?: string; to?: string }): Promise<ArchiveResponse> {
    const searchParams = new URLSearchParams();
    if (params?.from) searchParams.set('from', params.from);
    if (params?.to) searchParams.set('to', params.to);

    const queryString = searchParams.toString();
    const url = `/api/archive${queryString ? `?${queryString}` : ''}`;
    return apiFetch<ArchiveResponse>(url);
  },

  async getPromptTree(id: string): Promise<PromptTreeResponse> {
    return apiFetch<PromptTreeResponse>(`/api/prompts/${id}/tree`);
  },

  async listPromptSnippets(): Promise<PromptSnippet[]> {
    return apiFetch<PromptSnippet[]>('/api/prompt-snippets');
  },

  async createPromptSnippet(payload: CreatePromptSnippetRequest): Promise<PromptSnippet> {
    return apiFetch<PromptSnippet>('/api/prompt-snippets', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async replacePromptSnippets(payload: CreatePromptSnippetRequest[]): Promise<PromptSnippet[]> {
    return apiFetch<PromptSnippet[]>('/api/prompt-snippets', {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async updatePromptSnippet(id: string, patch: UpdatePromptSnippetRequest): Promise<PromptSnippet> {
    return apiFetch<PromptSnippet>(`/api/prompt-snippets/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    });
  },

  async deletePromptSnippet(id: string): Promise<void> {
    return apiFetch<void>(`/api/prompt-snippets/${id}`, {
      method: 'DELETE',
    });
  },

  async getNode(id: string): Promise<{ node: PromptNode<PromptFragment[]>; images: Image[]; results?: Result[] }> {
    return apiFetch<{ node: PromptNode<PromptFragment[]>; images: Image[]; results?: Result[] }>(`/api/nodes/${id}`);
  },

  async reroll(id: string, options?: { request_id?: string; aspect_ratio?: string; image_size?: string; model?: string }): Promise<GenerateResponse> {
    const body: { request_id?: string; aspect_ratio?: string; image_size?: string; model?: string } = {};
    if (options?.request_id) body.request_id = options.request_id;
    if (options?.aspect_ratio) body.aspect_ratio = options.aspect_ratio;
    if (options?.image_size) body.image_size = options.image_size;
    if (options?.model) body.model = options.model;
    return apiFetch<GenerateResponse>(`/api/prompts/${id}/reroll`, {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  async deleteResult(id: string): Promise<void> {
    return apiFetch<void>(`/api/results/${id}`, {
      method: 'DELETE',
    });
  },

  async deletePrompt(id: string): Promise<void> {
    return apiFetch<void>(`/api/prompts/${id}`, {
      method: 'DELETE',
    });
  },

  async deleteNode(id: string, mode: 'A' | 'B'): Promise<void> {
    const searchParams = new URLSearchParams();
    searchParams.set('mode', mode);
    return apiFetch<void>(`/api/nodes/${id}?${searchParams.toString()}`, {
      method: 'DELETE',
    });
  },
};

export async function getModels(): Promise<ModelInfo[]> {
  const res = await apiFetch<{ models: ModelInfo[] }>('/api/models');
  return res.models;
}
