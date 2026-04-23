import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { Img2ImgPage } from '../Img2ImgPage';
import { api } from '../../lib/api';

function LocationProbe() {
  const { search } = useLocation();
  return <div data-testid="location-search">{search}</div>;
}

vi.mock('../../lib/api', () => ({
  api: {
    generate: vi.fn(),
    reroll: vi.fn(),
    getPromptTree: vi.fn(),
    getNode: vi.fn(),
    upload: vi.fn(),
    listLibrary: vi.fn(),
    deleteResult: vi.fn(),
    deleteNode: vi.fn(),
    listPromptSnippets: vi.fn(),
  },
  getModels: vi.fn().mockResolvedValue([]),
}));

vi.mock('@xgfan/prompt-editor', () => ({
  PromptEditorSDK: ({ onSave, value }: any) => (
    <div data-testid="prompt-editor-sdk-modal">
      <button 
        data-testid="sdk-save-btn" 
        onClick={() => {
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
    </div>
  ),
}));

describe('Img2ImgPage UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.listLibrary as any).mockResolvedValue([]);
    (api.listPromptSnippets as any).mockResolvedValue([]);
    (api.getPromptTree as any).mockResolvedValue({ root_id: 'root-1', nodes: [] });
  });

  it('renders multi-reference image upload card', () => {
    render(
      <MemoryRouter>
        <Img2ImgPage />
      </MemoryRouter>
    );

    expect(screen.getByTestId('upload-card')).toBeInTheDocument();
  });

  it('uses reference_image_ids in generate payload', async () => {
    const mockImage = { id: 'img-1', file_path: '/path/1.png', created_at: '', origin: 'uploaded' };
    (api.upload as any).mockResolvedValue({ image: mockImage });
    (api.generate as any).mockResolvedValue({
      node: { id: 'node-1', prompt: '' },
      images: [],
      results: [],
    });

    render(
      <MemoryRouter>
        <Img2ImgPage />
      </MemoryRouter>
    );

    const fileInput = screen.getByTestId('file-input');
    fireEvent.change(fileInput, {
      target: { files: [new File([''], 'test1.png', { type: 'image/png' })] },
    });

    await waitFor(() => {
      expect(screen.getByAltText('ref-img-1')).toBeInTheDocument();
    });

    // Prompt is empty by default, so "Generate" might be disabled or allowed depending on logic.
    // Img2ImgPage: disabled={isGenerating} (doesn't check empty prompt explicitly in JSX, 
    // but handleGenerate checks reference images).
    // Prompt is required? handleGenerate checks:
    // if (sourceImages.length === 0) setError...
    // It does NOT check prompt empty for Img2Img (unlike Txt2Img).
    
    // We need to trigger a generate.
    const generateBtn = screen.getByTestId('generate');
    fireEvent.click(generateBtn);

    await waitFor(() => {
      expect(api.generate).toHaveBeenCalledWith(expect.objectContaining({
        reference_image_ids: ['img-1'],
      }));
    });
  });

  it('uses reroll endpoint when prompt not dirty and node selected', async () => {
    const mockImage = { id: 'img-1', file_path: '/path/1.png', created_at: '', origin: 'uploaded' };
    (api.upload as any).mockResolvedValue({ image: mockImage });
    (api.getNode as any).mockResolvedValue({
      node: { id: 'node-1', prompt: 'keep same', reference_image_ids: [] },
      images: [],
      results: [],
    });
    (api.reroll as any).mockResolvedValue({
      node: { id: 'node-1', prompt: 'keep same' },
      images: [],
      results: [],
    });

    render(
      <MemoryRouter initialEntries={['/?node_id=node-1']}>
        <Img2ImgPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(api.getNode).toHaveBeenCalledWith('node-1');
    });

    // Ensure prompt is loaded
    expect(screen.getByTestId('prompt')).toHaveValue('keep same');

    // To verify reroll, we must ensure NOT dirty.
    // Initial load: prompt = 'keep same', baseline = 'keep same'. isDirty = false.
    // We need at least one reference image to generate/reroll.
    
    const fileInput = screen.getByTestId('file-input');
    fireEvent.change(fileInput, {
      target: { files: [new File([''], 'test1.png', { type: 'image/png' })] },
    });

    await waitFor(() => {
      expect(screen.getByAltText('ref-img-1')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId('generate'));

    await waitFor(() => {
      expect(api.reroll).toHaveBeenCalledWith('node-1', {
        aspect_ratio: '1:1',
        image_size: '1K',
      });
      expect(api.generate).not.toHaveBeenCalled();
    });
  });
  
  it('updates prompt via SDK and switches to generate mode (dirty state)', async () => {
     (api.getNode as any).mockResolvedValue({
      node: { id: 'node-1', prompt: 'initial' },
      images: [{ id: 'img-1', file_path: '/init.png' }],
      results: [{ id: 'r-1', image_id: 'img-1' }],
    });
    
    render(
      <MemoryRouter initialEntries={['/?node_id=node-1']}>
        <Img2ImgPage />
      </MemoryRouter>
    );
    
    await waitFor(() => {
      expect(screen.getByTestId('prompt')).toHaveValue('initial');
    });

    fireEvent.click(screen.getByTestId('fork-prompt-btn'));
    await waitFor(() => expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument());

    // Save changes -> "new prompt"
    fireEvent.click(screen.getByText('Save SDK'));
    await waitFor(() => expect(screen.queryByTestId('prompt-editor-sdk-modal')).not.toBeInTheDocument());

    // Verify prompt updated
    expect(screen.getByTestId('prompt')).toHaveValue('new prompt');

    // Add reference image to allow generate
    const mockImage = { id: 'img-1', file_path: '/path/1.png', created_at: '', origin: 'uploaded' };
    (api.upload as any).mockResolvedValue({ image: mockImage });
    const fileInput = screen.getByTestId('file-input');
    fireEvent.change(fileInput, {
      target: { files: [new File([''], 'test1.png', { type: 'image/png' })] },
    });
    await waitFor(() => expect(screen.getByAltText('ref-img-1')).toBeInTheDocument());

    // Click generate
    fireEvent.click(screen.getByTestId('generate'));

    // Should call generate (because dirty: "new prompt" != "initial")
    await waitFor(() => {
        expect(api.generate).toHaveBeenCalledWith(expect.objectContaining({
            prompt: 'new prompt',
            parent_node_id: 'node-1'
        }));
        expect(api.reroll).not.toHaveBeenCalled();
    });
  });

  it('automatically opens PromptEditor modal when action=new is in URL', async () => {
    render(
      <MemoryRouter initialEntries={['/?action=new']}>
        <Img2ImgPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
    });
  });

  it('resets image_size when model change makes current size unsupported', async () => {
    const geminiModel = {
      id: 'gemini-3-pro-image',
      label: 'Gemini',
      allowed_sizes: ['1K', '2K', '4K'],
      allowed_aspect_ratios: ['1:1', '3:2', '21:9'],
      default_size: '1K',
    }
    const gptModel = {
      id: 'gpt-image-2',
      label: 'GPT Image 2',
      allowed_sizes: ['1K', '2K'],
      allowed_aspect_ratios: ['1:1', '3:2'],
      default_size: '1K',
    }
    const { getModels: mockGetModels } = await import('../../lib/api')
    ;(mockGetModels as ReturnType<typeof vi.fn>).mockResolvedValue([geminiModel, gptModel])

    render(
      <MemoryRouter>
        <Img2ImgPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByLabelText(/image size/i)).toBeInTheDocument()
    })

    // Select 4K (only allowed in gemini)
    fireEvent.change(screen.getByLabelText(/image size/i), { target: { value: '4K' } })
    expect(screen.getByLabelText(/image size/i)).toHaveValue('4K')

    // Switch to gpt-image-2 which only allows 1K/2K
    fireEvent.change(screen.getAllByRole('combobox')[0], { target: { value: 'gpt-image-2' } })

    // image_size should be reset to gpt-image-2's default_size
    await waitFor(() => {
      expect(screen.getByLabelText(/image size/i)).toHaveValue('1K')
    })
  })

  it('resets aspect_ratio when model change makes current ratio unsupported', async () => {
    const geminiModel = {
      id: 'gemini-3-pro-image',
      label: 'Gemini',
      allowed_sizes: ['1K', '2K', '4K'],
      allowed_aspect_ratios: ['1:1', '3:2', '21:9'],
      default_size: '1K',
    }
    const gptModel = {
      id: 'gpt-image-2',
      label: 'GPT Image 2',
      allowed_sizes: ['1K', '2K'],
      allowed_aspect_ratios: ['1:1', '3:2'],
      default_size: '1K',
    }
    const { getModels: mockGetModels } = await import('../../lib/api')
    ;(mockGetModels as ReturnType<typeof vi.fn>).mockResolvedValue([geminiModel, gptModel])

    render(
      <MemoryRouter>
        <Img2ImgPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByLabelText(/aspect ratio/i)).toBeInTheDocument()
    })

    // Select 21:9 (only allowed in gemini)
    fireEvent.change(screen.getByLabelText(/aspect ratio/i), { target: { value: '21:9' } })
    expect(screen.getByLabelText(/aspect ratio/i)).toHaveValue('21:9')

    // Switch to gpt-image-2 which doesn't allow 21:9
    fireEvent.change(screen.getAllByRole('combobox')[0], { target: { value: 'gpt-image-2' } })

    // aspect_ratio should be reset to first allowed ratio
    await waitFor(() => {
      expect(screen.getByLabelText(/aspect ratio/i)).toHaveValue('1:1')
    })
  })

  it('handles fork flow correctly: prefill, open modal, and maintain parent semantics', async () => {
    (api.getNode as any).mockResolvedValue({
      node: { id: 'source-id', prompt: 'source prompt', reference_image_ids: ['img-ref-1'] },
      images: [{ id: 'img-1', file_path: '/src.png' }],
      results: [{ id: 'r-1', image_id: 'img-1' }],
    });
    (api.generate as any).mockResolvedValue({
      node: { id: 'forked-id', prompt: 'forked prompt' },
      images: [],
      results: [],
    });
    (api.listLibrary as any).mockResolvedValue([
        { id: 'img-ref-1', file_path: '/path/1.png', created_at: '', origin: 'uploaded' }
    ]);

    render(
      <MemoryRouter initialEntries={['/?node_id=source-id']}>
        <LocationProbe />
        <Img2ImgPage />
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
