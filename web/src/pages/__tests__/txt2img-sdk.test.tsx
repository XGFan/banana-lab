import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from '../../lib/api';
import { MemoryRouter } from 'react-router-dom';

// Mock API before importing Txt2ImgPage
vi.mock('../../lib/api', () => ({
  api: {
    listPromptSnippets: vi.fn(),
    replacePromptSnippets: vi.fn(),
    createPromptSnippet: vi.fn(),
    updatePromptSnippet: vi.fn(),
    deletePromptSnippet: vi.fn(),
    generate: vi.fn(),
    getPromptTree: vi.fn(),
    getNode: vi.fn(),
    reroll: vi.fn(),
    deleteResult: vi.fn(),
    deleteNode: vi.fn(),
    getArchive: vi.fn(),
    listPrompts: vi.fn(),
  },
  getModels: vi.fn().mockResolvedValue([]),
  imageUrl: (img: any) => img?.file_path || '',
}));

vi.mock('@xgfan/prompt-editor', () => {
  const React = require('react');
  const { forwardRef, useImperativeHandle, useState } = React;
  
  return {
    PromptEditorSDK: forwardRef(({ onLibraryChange, value, initialLibrary, readOnly }: any, ref: any) => {
      const [fragments, setFragments] = useState(value?.fragments);
      const [shouldFail, setShouldFail] = useState(false);

      useImperativeHandle(ref, () => ({
        requestSave: async () => {
          if (shouldFail) {
            return { ok: false, error: { message: 'Mock validation error' }, value: fragments };
          }
          return { ok: true, value: fragments };
        }
      }));
  
      return (
        <div data-testid="prompt-editor-sdk-modal">
          {readOnly && <div data-testid="sdk-readonly-mode">ReadOnly Mode</div>}
          <button
            data-testid="sdk-set-value-btn"
            disabled={readOnly}
            onClick={() => {
               setShouldFail(false);
               setFragments({
                 groupOrder: ['g1'],
                 groups: { 'g1': { id: 'g1', name: 'Default', promptIds: ['p1'], collapsed: false } },
                 prompts: { 'p1': { id: 'p1', content: 'new sdk prompt', type: 'text', createdAt: '2024-01-01', updatedAt: '2024-01-01' } },
               });
            }}
          >
              Set Valid Value
          </button>
          <button
            data-testid="sdk-set-invalid-btn"
            disabled={readOnly}
            onClick={() => {
               setShouldFail(true);
            }}
          >
              Set Invalid Value
          </button>
          <button
              data-testid="sdk-add-lib-btn"
              disabled={readOnly}
              onClick={() => {
                  const newLibItem = {
                    groupOrder: ['new-snippet-id'],
                    groups: { 'new-snippet-id': { id: 'new-snippet-id', name: 'New Snippet', promptIds: ['p1'], collapsed: false } },
                    prompts: { 'p1': { id: 'p1', content: 'snippet content', type: 'text', createdAt: '2024-01-01', updatedAt: '2024-01-01' } }
                  };
                  
                  const currentLib = initialLibrary || { groupOrder: [], groups: {}, prompts: {} };
                  const newLibrary = {
                      ...currentLib,
                      groupOrder: [...currentLib.groupOrder, 'new-snippet-id'],
                      groups: { ...currentLib.groups, ...newLibItem.groups },
                      prompts: { ...currentLib.prompts, ...newLibItem.prompts }
                  };
                  onLibraryChange?.(newLibrary);
               }}
           >
               Add Lib Item
           </button>
        </div>
      );
    }),
  };
});

vi.mock('../../components/ImagePreviewModal', () => ({
  ImagePreviewModal: () => <div data-testid="image-preview-modal" />,
}));

// Import Txt2ImgPage AFTER mocks
import { Txt2ImgPage } from '../Txt2ImgPage';

describe('Txt2ImgPage SDK Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.listPromptSnippets as any).mockResolvedValue([]);
    (api.replacePromptSnippets as any).mockImplementation(async (payload: any) => payload);
    (api.getPromptTree as any).mockResolvedValue({});
    (api.getNode as any).mockResolvedValue({ node: { id: 'test-node', prompt: 'test' }, images: [], results: [] });
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('opens SDK modal when edit button is clicked', async () => {
    render(
      <MemoryRouter>
        <Txt2ImgPage />
      </MemoryRouter>
    );

    const editBtn = screen.getByTestId('edit-prompt-btn');
    expect(editBtn).toBeInTheDocument();

    fireEvent.click(editBtn);

    await waitFor(() => {
        expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
    });
  });

  it('updates prompt when saved successfully from SDK', async () => {
    render(
      <MemoryRouter>
        <Txt2ImgPage />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('edit-prompt-btn'));
    await waitFor(() => expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument());

    fireEvent.click(screen.getByTestId('sdk-set-value-btn'));

    const saveBtn = screen.getByText('Save Changes');
    fireEvent.click(saveBtn);

    await waitFor(() => {
        expect(screen.queryByTestId('prompt-editor-sdk-modal')).not.toBeInTheDocument();
    });

    const textarea = screen.getByTestId('prompt');
    await waitFor(() => {
      expect(textarea).toHaveValue('new sdk prompt');
    });
  });

  it('displays error message when SDK save fails', async () => {
    render(
        <MemoryRouter>
          <Txt2ImgPage />
        </MemoryRouter>
      );
  
      fireEvent.click(screen.getByTestId('edit-prompt-btn'));
      await waitFor(() => expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument());
  
      fireEvent.click(screen.getByTestId('sdk-set-invalid-btn'));
  
      const saveBtn = screen.getByText('Save Changes');
      fireEvent.click(saveBtn);
  
      await waitFor(() => {
          expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
          expect(screen.getByTestId('sdk-save-error')).toHaveTextContent('Mock validation error');
      });
  });

  it('syncs library changes to backend', async () => {
    render(
      <MemoryRouter>
        <Txt2ImgPage />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('edit-prompt-btn'));

    await waitFor(() => {
        expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId('sdk-add-lib-btn'));

    await waitFor(() => {
        expect(api.replacePromptSnippets).toHaveBeenCalledWith(
          expect.arrayContaining([
            expect.objectContaining({
              name: 'New Snippet',
              content_fragments: [{ type: 'text', text: 'snippet content' }],
            }),
          ]),
        );
    });
  });

  it('opens SDK modal in readOnly mode when view button is clicked', async () => {
    (api.getNode as any).mockResolvedValue({ 
      node: { id: 'test-node', prompt: 'test prompt' }, 
      images: [{ id: 'img-1', file_path: '/test.png' }], 
      results: [{ id: 'r-1', image_id: 'img-1' }] 
    });

    render(
      <MemoryRouter initialEntries={['/?node_id=test-node']}>
        <Txt2ImgPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('view-prompt-btn')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId('view-prompt-btn'));

    await waitFor(() => {
      expect(screen.getByTestId('prompt-editor-sdk-modal')).toBeInTheDocument();
      expect(screen.getByTestId('sdk-readonly-mode')).toBeInTheDocument();
      expect(screen.queryByTestId('sdk-save-btn')).not.toBeInTheDocument();
      expect(screen.getByText('Close')).toBeInTheDocument();
    });
  });
});
