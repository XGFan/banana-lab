import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { Txt2ImgPage } from '../Txt2ImgPage';
import { api } from '../../lib/api';

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-search">{location.search}</div>;
}

// Mock API
vi.mock('../../lib/api', () => ({
  api: {
    generate: vi.fn(),
    reroll: vi.fn(),
    getPromptTree: vi.fn(),
    getNode: vi.fn(),
    deleteResult: vi.fn(),
    deleteNode: vi.fn(),
    listPromptSnippets: vi.fn(),
    createPromptSnippet: vi.fn(),
    updatePromptSnippet: vi.fn(),
    deletePromptSnippet: vi.fn(),
  },
  imageUrl: (img: any) => img?.file_path || '',
}));

// Mock SDK with correct PromptCollectionState structure
vi.mock('@xgfan/prompt-editor', () => ({
  PromptEditorSDK: ({ onSave, onCancel, value }: any) => (
    <div data-testid="prompt-editor-sdk-modal">
      <button 
        data-testid="sdk-save-btn" 
        onClick={() => {
          // Simulate saving with a modified prompt (via fragments)
          const newValue = { 
            ...value, 
            fragments: { 
              groupOrder: ['g1'],
              groups: { 'g1': { id: 'g1', name: 'Default', promptIds: ['p1'], collapsed: false } },
              prompts: { 'p1': { id: 'p1', content: 'new prompt', type: 'text', createdAt: '2024-01-01', updatedAt: '2024-01-01' } }
            } 
          };
          onSave(newValue.fragments);
        }}
      >
        Save SDK
      </button>
      <button onClick={onCancel}>Cancel SDK</button>
    </div>
  ),
}));

// Mock ImagePreviewModal
vi.mock('../../components/ImagePreviewModal', () => ({
  ImagePreviewModal: () => <div data-testid="image-preview-modal" />,
}));

describe('Txt2ImgPage UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getPromptTree as any).mockResolvedValue({ root_id: 'root-1', nodes: [] });
    (api.listPromptSnippets as any).mockResolvedValue([]);
  });

  it('renders size and ratio selectors', () => {
    render(
      <MemoryRouter>
        <Txt2ImgPage />
      </MemoryRouter>
    );

    expect(screen.getByLabelText(/aspect ratio/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/image size/i)).toBeInTheDocument();
  });

  it('always shows Generate button but handles branching based on dirty state', async () => {
    render(
      <MemoryRouter>
        <Txt2ImgPage />
      </MemoryRouter>
    );

    const generateBtn = screen.getByTestId('generate');
    expect(generateBtn).toBeInTheDocument();
    expect(generateBtn).toHaveTextContent('Generate');

    // Open SDK
    fireEvent.click(screen.getByTestId('edit-prompt-btn'));
    
    // Wait for modal
    await waitFor(() => {
        expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
    });

    // Save changes
    fireEvent.click(screen.getByText('Save SDK'));

    // Wait for modal to close
    await waitFor(() => {
        expect(screen.queryByTestId('prompt-editor-sdk-modal')).not.toBeInTheDocument();
    });

    await waitFor(() => {
        const btn = screen.getByTestId('generate');
        expect(btn).toBeInTheDocument();
        expect(btn).toHaveTextContent('Generate');
    });
  });

  it('switches URL node_id to new node after generate from historical node', async () => {
    (api.getNode as any).mockResolvedValue({
      node: { id: 'node-old', prompt: 'old prompt', aspect_ratio: '1:1' },
      images: [{ id: 'img-1', file_path: '/old.png' }],
      results: [{ id: 'r-1', image_id: 'img-1' }],
    });
    (api.generate as any).mockResolvedValue({
      node: { id: 'node-new', prompt: 'new prompt' },
      images: [],
      results: [],
    });

    render(
      <MemoryRouter initialEntries={['/?node_id=node-old']}>
        <LocationProbe />
        <Txt2ImgPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(api.getNode).toHaveBeenCalledWith('node-old');
    });

    fireEvent.click(screen.getByTestId('fork-prompt-btn'));
    
    await waitFor(() => {
        expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Save SDK')); // Sets prompt to "new prompt"

    await waitFor(() => {
        expect(screen.queryByTestId('prompt-editor-sdk-modal')).not.toBeInTheDocument();
        expect(screen.getByTestId('generate')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId('generate'));

    await waitFor(() => {
      expect(api.generate).toHaveBeenCalledWith(expect.objectContaining({ parent_node_id: 'node-old' }));
      expect(screen.getByTestId('location-search')).toHaveTextContent('node_id=node-new');
    });
  });

  it('keeps URL node_id unchanged after reroll', async () => {
    (api.getNode as any).mockResolvedValue({
      node: { id: 'node-old', prompt: 'old prompt', aspect_ratio: '1:1' },
      images: [],
      results: [],
    });
    (api.reroll as any).mockResolvedValue({
      node: { id: 'node-old', prompt: 'old prompt' },
      images: [],
      results: [],
    });

    render(
      <MemoryRouter initialEntries={['/?node_id=node-old']}>
        <LocationProbe />
        <Txt2ImgPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(api.getNode).toHaveBeenCalledWith('node-old');
    });

    // Reroll is enabled when prompt is not dirty (matches baseline)
    // Initial load sets baseline = prompt = 'old prompt'.
    // So 'reroll' button should be visible.
    
    fireEvent.click(screen.getByTestId('generate'));

    await waitFor(() => {
      expect(api.reroll).toHaveBeenCalledWith('node-old', {
        aspect_ratio: '1:1',
        image_size: '1K',
      });
      expect(screen.getByTestId('location-search')).toHaveTextContent('node_id=node-old');
    });
  });

  it('clears selected state when tree lookup returns 404 not_found', async () => {
    const err = Object.assign(new Error('not found'), { status: 404, code: 'not_found' });
    (api.getPromptTree as any).mockRejectedValue(err);

    render(
      <MemoryRouter initialEntries={['/?node_id=invalid-node']}>
        <LocationProbe />
        <Txt2ImgPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('location-search')).toHaveTextContent('');
      expect(screen.queryByTestId('generate-error')).not.toBeInTheDocument();
      expect(api.getNode).not.toHaveBeenCalled();
    });
  });

  it('automatically opens PromptEditor modal when action=new is in URL', async () => {
    render(
      <MemoryRouter initialEntries={['/?action=new']}>
        <Txt2ImgPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
    });
  });

  it('handles fork flow correctly: prefill, open modal, and maintain parent semantics', async () => {
    (api.getNode as any).mockResolvedValue({
      node: { id: 'source-id', prompt: 'source prompt', aspect_ratio: '1:1' },
      images: [{ id: 'img-1', file_path: '/src.png' }],
      results: [{ id: 'r-1', image_id: 'img-1' }],
    });
    (api.generate as any).mockResolvedValue({
      node: { id: 'forked-id', prompt: 'forked prompt' },
      images: [],
      results: [],
    });

    render(
      <MemoryRouter initialEntries={['/?node_id=source-id']}>
        <LocationProbe />
        <Txt2ImgPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(api.getNode).toHaveBeenCalledWith('source-id');
    });

    fireEvent.click(screen.getByTestId('fork-prompt-btn'));

    await waitFor(() => {
      expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
      const search = screen.getByTestId('location-search').textContent;
      expect(search).not.toContain('action=fork');
      expect(search).not.toContain('node_id=source-id');
    });

    fireEvent.click(screen.getByText('Save SDK'));

    await waitFor(() => {
      expect(screen.queryByTestId('prompt-editor-sdk-modal')).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId('generate'));

    await waitFor(() => {
      expect(api.generate).toHaveBeenCalledWith(expect.objectContaining({ 
        parent_node_id: 'source-id',
        prompt: 'new prompt' 
      }));
    });
  });
});
