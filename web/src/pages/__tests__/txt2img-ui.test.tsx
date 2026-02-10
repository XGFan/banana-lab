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
vi.mock('prompt-editor-sdk', () => ({
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

  it('toggles between Re-roll and Generate buttons based on dirty state', async () => {
    render(
      <MemoryRouter>
        <Txt2ImgPage />
      </MemoryRouter>
    );

    expect(screen.getByTestId('reroll')).toBeInTheDocument();
    expect(screen.queryByTestId('generate')).not.toBeInTheDocument();

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

    // Now prompt should be 'new prompt', which is different from baseline (empty)
    await waitFor(() => {
        expect(screen.getByTestId('generate')).toBeInTheDocument();
        expect(screen.queryByTestId('reroll')).not.toBeInTheDocument();
    });
  });

  it('switches URL node_id to new node after generate from historical node', async () => {
    (api.getNode as any).mockResolvedValue({
      node: { id: 'node-old', prompt: 'old prompt', aspect_ratio: '1:1' },
      images: [],
      results: [],
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

    // Open SDK and change prompt
    fireEvent.click(screen.getByTestId('edit-prompt-btn'));
    
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
    
    fireEvent.click(screen.getByTestId('reroll'));

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
});
