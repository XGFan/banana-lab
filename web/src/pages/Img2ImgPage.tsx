import { useState, useRef, useEffect, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { api, type ApiError, type Image, type Result, type PromptFragment, type PromptSnippet } from '../lib/api';
import { imageUrl } from '../lib/imageUrl';
import { LibraryModal, type Filters } from '../components/LibraryModal';
import { buildImg2ImgPayload } from '../lib/generatePayload';
import { imageSizes, aspectRatios } from '../lib/imagePresets';
import { ImagePreviewModal } from '../components/ImagePreviewModal';
import { generateButtonClass } from '../lib/buttonStyles';
import { PromptEditorSDK, type PromptEditorValue, type PromptEditorSDKHandle, type PromptEditorLibrary } from '@xgfan/prompt-editor';
import '@xgfan/prompt-editor/style.css';
import { toSdkValue, collectionToPlainPrompt, collectionFromPlainPrompt, snippetsFromLibrary, libraryFromSnippets } from '../lib/promptEditor/adapter';
import type { PromptCollectionState } from '../lib/promptEditor/types';

const EMPTY_COLLECTION: PromptCollectionState = {
  groupOrder: [],
  groups: {},
  prompts: {},
  version: 1,
};

export function Img2ImgPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const skipNextClearRef = useRef(false);
  const lastHandledActionRef = useRef<string | null>(null);
  const [prompt, setPrompt] = useState('');
  const [fragments, setFragments] = useState<PromptCollectionState | PromptFragment[]>([]);
  const [baselinePrompt, setBaselinePrompt] = useState('');
  const [sourceImages, setSourceImages] = useState<Image[]>([]);
  const [generatedImages, setGeneratedImages] = useState<Image[]>([]);
  const [results, setResults] = useState<Result[]>([]);
  const [isGenerating, setIsGenerating] = useState(false);
  const [currentNodeId, setCurrentNodeId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [aspectRatio, setAspectRatio] = useState('1:1');
  const [imageSize, setImageSize] = useState('1K');
  const isDirty = prompt.trim() !== baselinePrompt.trim();
  const [previewImage, setPreviewImage] = useState<Image | null>(null);
  const [showDeleteNodeModal, setShowDeleteNodeModal] = useState(false);

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isReadOnly, setIsReadOnly] = useState(false);
  const [sdkValue, setSdkValue] = useState<PromptEditorValue | null>(null);
  const [sdkLibrary, setSdkLibrary] = useState<PromptEditorLibrary>(libraryFromSnippets([]));
  const [saveError, setSaveError] = useState<string | null>(null);

  const [libraryOpen, setLibraryOpen] = useState(false);
  const [libraryImages, setLibraryImages] = useState<Image[]>([]);
  const [libraryLoading, setLibraryLoading] = useState(false);
  const [libraryPage, setLibraryPage] = useState(1);
  const [libraryHasMore, setLibraryHasMore] = useState(true);
  const [libraryFilters, setLibraryFilters] = useState<Filters>({
    query: '',
    date: 'all',
    type: 'all'
  });
  const [librarySelectedId, setLibrarySelectedId] = useState<string | null>(null);

  const sdkRef = useRef<PromptEditorSDKHandle>(null);
  const librarySnippetsRef = useRef<PromptSnippet[]>([]);
  const librarySyncQueueRef = useRef<Promise<void>>(Promise.resolve());
  const fileInputRef = useRef<HTMLInputElement>(null);

  const clearSelectedNode = useCallback(() => {
    setCurrentNodeId(null);
    setPrompt('');
    setFragments(EMPTY_COLLECTION);
    setBaselinePrompt('');
    setAspectRatio('1:1');
    setImageSize('1K');
    setGeneratedImages([]);
    setResults([]);
    setSourceImages([]);
  }, []);

  const handleOpenEditor = useCallback(async (options?: { initialPrompt?: string; initialFragments?: any; readOnly?: boolean }) => {
    const openWith = (snippets: PromptSnippet[]) => {
      librarySnippetsRef.current = snippets;
      const p = options?.initialPrompt !== undefined ? options.initialPrompt : prompt;
      const f = options?.initialFragments !== undefined ? options.initialFragments : fragments;
      const initialValue = toSdkValue(p, f, snippets);
      setSdkValue(initialValue.value);
      setSdkLibrary(initialValue.library);
      setSaveError(null);
      setIsReadOnly(!!options?.readOnly);
      setIsModalOpen(true);
    };
    try {
      openWith(await api.listPromptSnippets());
    } catch (err) {
      console.error('Failed to load library:', err);
      setError('Failed to load snippet library');
      openWith([]);
    }
  }, [prompt, fragments]);

  const handleFork = useCallback(() => {
    if (!currentNodeId) return;
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.set('action', 'fork');
      next.set('sourceNodeId', currentNodeId);
      next.set('sourcePrompt', prompt);
      return next;
    });
  }, [currentNodeId, prompt, setSearchParams]);

  useEffect(() => {
    const action = searchParams.get('action');
    if (!action) {
      lastHandledActionRef.current = null;
      return;
    }

    if (action === lastHandledActionRef.current) return;
    lastHandledActionRef.current = action;

    if (action === 'new') {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.delete('action');
        return next;
      }, { replace: true });
      clearSelectedNode();
      void handleOpenEditor({ initialPrompt: '', initialFragments: EMPTY_COLLECTION });
    } else if (action === 'fork') {
      const sourceNodeId = searchParams.get('sourceNodeId');
      const sourcePrompt = searchParams.get('sourcePrompt');
      
      if (sourceNodeId && sourcePrompt) {
        skipNextClearRef.current = true;
        setSearchParams((prev) => {
          const next = new URLSearchParams(prev);
          next.delete('action');
          next.delete('sourceNodeId');
          next.delete('sourcePrompt');
          next.delete('node_id');
          return next;
        }, { replace: true });
        
        setPrompt(sourcePrompt);
        const forkFragments = collectionFromPlainPrompt(sourcePrompt);
        setFragments(forkFragments);
        setCurrentNodeId(sourceNodeId);
        setBaselinePrompt('');
        setGeneratedImages([]);
        setResults([]);

        void handleOpenEditor({ initialPrompt: sourcePrompt, initialFragments: forkFragments });
      }
    }
  }, [searchParams, setSearchParams, handleOpenEditor, clearSelectedNode]);

  useEffect(() => {
    const action = searchParams.get('action');
    const nid = searchParams.get('node_id');

    if (action) return;

    const restoreImages = async (ids: string[]) => {
      if (!ids || ids.length === 0) return;
      
      const restored: Image[] = ids.map(id => ({
        id,
        file_path: '',
        origin: 'uploaded',
        created_at: ''
      }));
      setSourceImages(restored);

      try {
        const library = await api.listLibrary({ page: 1, limit: 100 });
        const detailed = restored.map(placeholder => {
          const found = library.find(img => img.id === placeholder.id);
          return found || placeholder;
        });
        setSourceImages(detailed);
      } catch {
      }
    };

    if (nid) {
      api.getPromptTree(nid).then(() => api.getNode(nid)).then(async ({ node, images, results }) => {
        const restoredAspectRatio = node.aspect_ratio ?? '1:1';
        const restoredImageSize = node.image_size ?? '1K';
        setCurrentNodeId(node.id);
        setPrompt(node.prompt);
        setFragments(node.fragments || []);
        setBaselinePrompt(node.prompt);
        setAspectRatio(restoredAspectRatio);
        setImageSize(restoredImageSize);
        setGeneratedImages(images);
        setResults(results || []);
        if (node.reference_image_ids) {
          await restoreImages(node.reference_image_ids);
        }
      }).catch((err: unknown) => {
        const apiError = err as ApiError;
        if (apiError?.status === 404 && apiError?.code === 'not_found') {
          setSearchParams((prev) => {
            const next = new URLSearchParams(prev);
            next.delete('node_id');
            return next;
          }, { replace: true });
          clearSelectedNode();
          setError(null);
          return;
        }
        setError('Failed to restore node state');
      });
    } else {
      if (skipNextClearRef.current) {
        skipNextClearRef.current = false;
      } else {
        clearSelectedNode();
      }
    }
  }, [searchParams, setSearchParams, clearSelectedNode]);

  const handleLoadMore = () => {
    if (libraryLoading || !libraryHasMore) return;

    setLibraryLoading(true);
    const nextPage = libraryPage + 1;

    api.listLibrary({
      page: nextPage,
      limit: 60,
      type: libraryFilters.type === 'all' ? undefined : libraryFilters.type,
      date: libraryFilters.date === 'all' ? undefined : libraryFilters.date,
    })
      .then(imgs => {
        setLibraryImages(prev => {
          const existingIds = new Set(prev.map(i => i.id));
          const uniqueNew = imgs.filter(i => !existingIds.has(i.id));
          return [...prev, ...uniqueNew];
        });
        setLibraryHasMore(imgs.length === 60);
        setLibraryPage(nextPage);
      })
      .catch(() => setError('Failed to load more library'))
      .finally(() => setLibraryLoading(false));
  };

  const handleUploadClick = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (sourceImages.length >= 5) {
        setError('Maximum 5 reference images allowed');
        return;
    }

    try {
      const { image } = await api.upload(file);
      setSourceImages(prev => [...prev, image]);
      setError(null);
    } catch (err) {
      console.error('Upload failed', err);
      setError('Failed to upload image');
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleRemoveImage = (id: string) => {
      setSourceImages(prev => prev.filter(img => img.id !== id));
  };

  const handleGenerate = async () => {
    if (sourceImages.length === 0) {
      setError('Please select at least one reference image');
      return;
    }

    setIsGenerating(true);
    setError(null);

    try {
      const res = (!isDirty && currentNodeId)
        ? await api.reroll(currentNodeId, {
            aspect_ratio: aspectRatio,
            image_size: imageSize,
          })
        : await api.generate(buildImg2ImgPayload({
            parentNodeId: currentNodeId || undefined,
            prompt,
            fragments,
            referenceImageIds: sourceImages.map(img => img.id),
            aspectRatio,
            imageSize,
          }));

      setCurrentNodeId(res.node.id);
      setPrompt(res.node.prompt);
      setFragments(res.node.fragments || []);
      setBaselinePrompt(res.node.prompt);
      setGeneratedImages(res.images);
      setResults(res.results || []);
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('node_id', res.node.id);
        return next;
      }, { replace: true });
    } catch (err: unknown) {
      console.error('Generation failed', err);
      const message = err instanceof Error ? err.message : '';
      if (message.includes('busy')) {
        setError('系统繁忙');
      } else {
        setError('生成失败');
      }
    } finally {
      setIsGenerating(false);
    }
  };

  const handleDeleteResult = async (resultId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!window.confirm('Are you sure you want to delete this result?')) return;

    try {
      await api.deleteResult(resultId);
      setResults(prev => prev.filter(r => r.id !== resultId));
    } catch (err) {
      alert('Failed to delete result');
    }
  };

  const handleDeleteNode = async (mode: 'A' | 'B') => {
    if (!currentNodeId) return;
    
    try {
      await api.deleteNode(currentNodeId, mode);
      setShowDeleteNodeModal(false);

      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.delete('node_id');
        return next;
      });
      clearSelectedNode();
    } catch (err) {
      alert('Failed to delete prompt node');
    }
  };

  const filteredLibraryImages = libraryImages.filter(img => {
    if (!libraryFilters.query) return true;
    const q = libraryFilters.query.toLowerCase();
    return img.id.toLowerCase().includes(q) || img.file_path.toLowerCase().includes(q);
  });

  const getGridCols = () => {
      const count = sourceImages.length + 1;
      if (count === 1) return 'grid-cols-1';
      if (sourceImages.length === 1) return 'grid-cols-2';
      if (sourceImages.length >= 4) return 'grid-cols-3';
      return 'grid-cols-2';
  };

  useEffect(() => {
    if (libraryOpen) {
      setLibraryLoading(true);
      setLibraryImages([]);
      setLibraryPage(1);
      setLibraryHasMore(true);
      
      api.listLibrary({
        page: 1,
        limit: 60,
        type: libraryFilters.type === 'all' ? undefined : libraryFilters.type,
        date: libraryFilters.date === 'all' ? undefined : libraryFilters.date,
      })
        .then(imgs => {
          setLibraryImages(imgs);
          setLibraryHasMore(imgs.length === 60);
        })
        .catch(() => setError('Failed to load library'))
        .finally(() => setLibraryLoading(false));
    }
  }, [libraryOpen, libraryFilters.type, libraryFilters.date]);

  const handleLibraryChange = async (nextLibrary: PromptEditorLibrary) => {
    setSdkLibrary(nextLibrary);
    const nextSnippets = snippetsFromLibrary(nextLibrary);

    librarySyncQueueRef.current = librarySyncQueueRef.current
      .then(async () => {
        const replaced = await api.replacePromptSnippets(
          nextSnippets.map((item) => ({
            id: item.id,
            name: item.name,
            content_fragments: item.content_fragments,
          })),
        );
        librarySnippetsRef.current = replaced;
      })
      .catch((e) => {
        console.error('Failed to sync snippet library:', e);
        setError('Failed to sync snippet library');
      });

    await librarySyncQueueRef.current;
  };

  const handleSaveClick = async () => {
      if (!sdkRef.current) return;
      const result = await sdkRef.current.requestSave();
      if (!result) return;

      if (result.ok) {
          handleSdkSave(result.value);
      } else {
          setSaveError(result.error.message);
      }
  };

  // SDK onSave actually passes { fragments, promptText } at runtime,
  // but its type declaration still says PromptCollectionState — accept both shapes.
  const handleSdkSave = (saveResult: unknown) => {
      const data = saveResult as Record<string, unknown>;
      if (data && typeof data === 'object' && 'promptText' in data && 'fragments' in data) {
          const promptText = data.promptText as string;
          const nextCollection = data.fragments as PromptCollectionState;
          setPrompt(promptText);
          setFragments(nextCollection);
      } else {
          const nextCollection = saveResult as PromptCollectionState;
          const nextPrompt = collectionToPlainPrompt(nextCollection);
          setPrompt(nextPrompt);
          setFragments(nextCollection);
      }
      setIsModalOpen(false);
  };

  const handleE2eApply = (text: string) => {
      setSdkValue(prev => {
          if (!prev) return prev;
          const fragments = collectionFromPlainPrompt(text);
          return {
              ...prev,
              fragments,
          };
      });
      if (saveError) setSaveError(null);
  };

  const displayItems: (Image & { resultId?: string })[] = results.length > 0 
    ? results.map(r => {
        const img = generatedImages.find(i => i.id === r.asset_id);
        return img ? { ...img, resultId: r.id } : null;
      }).filter((item): item is Image & { resultId: string } => item !== null)
    : generatedImages.map(img => ({ ...img, resultId: undefined }));

  return (
    <div className="flex flex-col h-full w-full bg-slate-50 text-slate-900 font-sans">
      <input
        type="file"
        ref={fileInputRef}
        data-testid="file-input"
        className="hidden"
        accept="image/*"
        onChange={handleFileChange}
      />

      <ImagePreviewModal 
        image={previewImage} 
        onClose={() => setPreviewImage(null)} 
      />

      {isModalOpen && sdkValue && (
        <div data-testid="sdk-modal-overlay" className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-white rounded-xl shadow-xl w-[90vw] h-[80vh] flex flex-col overflow-hidden">
              <PromptEditorSDK
                ref={sdkRef}
                value={sdkValue}
                initialLibrary={sdkLibrary}
                onLibraryChange={handleLibraryChange}
                onSave={handleSdkSave}
                onCancel={() => setIsModalOpen(false)}
                readOnly={isReadOnly}
              />
             <div className="p-4 border-t border-slate-200 flex justify-between items-center gap-2 bg-slate-50">
                <div className="flex-1 text-sm text-red-600 font-medium px-2">
                    {saveError && <span data-testid="sdk-save-error">{saveError}</span>}
                </div>
                <div className="flex gap-2">
                    <button
                        type="button"
                        onClick={() => setIsModalOpen(false)}
                        className="px-4 py-2 text-sm font-medium text-slate-600 hover:text-slate-800 hover:bg-slate-100 rounded-lg transition-colors"
                    >
                        {isReadOnly ? 'Close' : 'Cancel'}
                    </button>
                    {!isReadOnly && (
                        <button
                            type="button"
                            data-testid="sdk-save-btn"
                            onClick={handleSaveClick}
                            className="px-4 py-2 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg transition-colors shadow-sm"
                        >
                            Save Changes
                        </button>
                    )}
                </div>
                {import.meta.env.VITE_E2E === '1' && (
                    <div className="absolute bottom-4 left-4 flex gap-2">
                        <input 
                            data-testid="e2e-sdk-prompt" 
                            className="border border-slate-300 rounded px-2 py-1 text-xs"
                            placeholder="E2E Prompt"
                            onChange={(e) => handleE2eApply(e.target.value)}
                        />
                        <button
                            type="button"
                            data-testid="e2e-sdk-apply"
                            className="bg-gray-200 px-2 py-1 text-xs rounded"
                            onClick={() => {}} 
                        >
                            Apply
                        </button>
                    </div>
                )}
             </div>
          </div>
        </div>
      )}



      {showDeleteNodeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
          <div className="bg-white rounded-xl shadow-xl max-w-md w-full p-6 animate-in fade-in zoom-in-95 duration-200">
            <h3 className="text-lg font-bold text-slate-900 mb-2">Delete Prompt Node</h3>
            <p className="text-slate-600 text-sm mb-6">
              How would you like to delete this prompt node?
            </p>
            
            <div className="flex flex-col gap-3">
              <button
                type="button"
                data-testid="delete-mode-a"
                onClick={() => handleDeleteNode('A')}
                className="flex items-start gap-3 p-3 rounded-lg border border-slate-200 hover:border-red-300 hover:bg-red-50 transition-colors text-left group"
              >
                <div className="p-2 bg-red-100 text-red-600 rounded-md group-hover:bg-red-200">
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                  </svg>
                </div>
                <div>
                  <div className="font-semibold text-slate-900 group-hover:text-red-700">Soft Delete (Mode A)</div>
                  <div className="text-xs text-slate-500 mt-1">Remove from result list. Images remain in storage.</div>
                </div>
              </button>

              <button
                type="button"
                data-testid="delete-mode-b"
                onClick={() => handleDeleteNode('B')}
                className="flex items-start gap-3 p-3 rounded-lg border border-slate-200 hover:border-amber-300 hover:bg-amber-50 transition-colors text-left group"
              >
                <div className="p-2 bg-amber-100 text-amber-600 rounded-md group-hover:bg-amber-200">
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="m20.25 7.5-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" />
                  </svg>
                </div>
                <div>
                  <div className="font-semibold text-slate-900 group-hover:text-amber-700">Archive (Mode B)</div>
                  <div className="text-xs text-slate-500 mt-1">Move to global archive. Hidden from session view.</div>
                </div>
              </button>
            </div>

            <div className="mt-6 flex justify-end">
              <button
                type="button"
                onClick={() => setShowDeleteNodeModal(false)}
                className="px-4 py-2 text-sm font-medium text-slate-600 hover:text-slate-800 hover:bg-slate-100 rounded-lg transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      <LibraryModal
        open={libraryOpen}
        images={filteredLibraryImages}
        selectedId={librarySelectedId}
        onSelect={setLibrarySelectedId}
        onCancel={() => setLibraryOpen(false)}
        onConfirm={() => {
          const selected = libraryImages.find(img => img.id === librarySelectedId);
          if (selected) {
            if (sourceImages.length < 5) {
                setSourceImages(prev => [...prev, selected]);
                setError(null);
            } else {
                setError('Maximum 5 reference images allowed');
            }
          }
          setLibraryOpen(false);
        }}
        filters={libraryFilters}
        onChangeFilters={setLibraryFilters}
        loading={libraryLoading}
        onLoadMore={handleLoadMore}
        hasMore={libraryHasMore}
      />

      <div className="px-6 py-3 border-b border-slate-200 flex items-center space-x-2 text-sm text-slate-500" data-testid="breadcrumbs">
        <span className="hover:text-slate-700 cursor-pointer transition-colors">Home</span>
        <span>/</span>
        <span className="text-slate-700 font-medium">Image to Image</span>
      </div>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="max-w-6xl mx-auto flex flex-col gap-8">
          
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg text-sm">
              {error}
            </div>
          )}

          <div className="grid grid-cols-2 gap-6">
            
            <div className="flex flex-col gap-2">
              <label className="text-xs font-bold uppercase tracking-wider text-slate-500">
                  Reference Images ({sourceImages.length}/5)
              </label>
              
              <div className={`grid gap-3 ${getGridCols()}`}>
                  {sourceImages.map((img, idx) => (
                      <div 
                        key={img.id + idx} 
                        className="relative aspect-square rounded-xl overflow-hidden border border-slate-200 bg-white group shadow-sm"
                        data-testid="reference-image"
                      >
                          <img 
                            src={imageUrl(img)} 
                            alt={`ref-img-${idx+1}`} 
                            className="w-full h-full object-cover"
                          />
                          <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                              <button
                                data-testid="delete-ref-image"
                                onClick={() => handleRemoveImage(img.id)}
                                className="bg-red-500 hover:bg-red-600 text-white p-2 rounded-full transition-transform hover:scale-110 shadow-lg"
                                title="Remove image"
                              >
                                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                              </button>
                          </div>
                      </div>
                  ))}
                  
                  {sourceImages.length < 5 && (
                      <div 
                        data-testid="upload-card"
                        className="aspect-square rounded-xl border-2 border-dashed border-slate-300 hover:border-indigo-400 hover:bg-slate-50 transition-all flex flex-col items-center justify-center cursor-pointer gap-2 group bg-white/50"
                      >
                          <div className="flex flex-col gap-2 items-center w-full px-4">
                              <button 
                                onClick={handleUploadClick}
                                className="w-full py-2 px-3 bg-white border border-slate-200 rounded-lg text-sm font-medium text-slate-700 shadow-sm hover:text-indigo-600 hover:border-indigo-200 transition-colors"
                              >
                                  Upload
                              </button>
                              <button 
                                onClick={() => setLibraryOpen(true)}
                                className="w-full py-2 px-3 bg-white border border-slate-200 rounded-lg text-sm font-medium text-slate-700 shadow-sm hover:text-indigo-600 hover:border-indigo-200 transition-colors"
                              >
                                  Library
                              </button>
                          </div>
                      </div>
                  )}
              </div>
            </div>

            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-2 h-full">
                <div className="flex justify-between items-center">
                   <label htmlFor="prompt-input" className="text-xs font-bold uppercase tracking-wider text-slate-500">Prompt</label>
                   {currentNodeId && (
                     <button
                       type="button"
                       data-testid="prompt-delete-button"
                       onClick={() => setShowDeleteNodeModal(true)}
                       className="text-slate-400 hover:text-red-500 transition-colors p-1 rounded hover:bg-red-50"
                       title="Delete Prompt Node"
                     >
                       <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-3.5 h-3.5" aria-hidden="true">
                         <path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                       </svg>
                     </button>
                   )}
                </div>
                <div className="relative group/prompt flex-1 flex flex-col">
                  <textarea 
                    id="prompt-input"
                    data-testid="prompt"
                    value={prompt}
                    readOnly
                    className="flex-1 w-full bg-white border border-slate-200 rounded-lg p-5 text-slate-900 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent min-h-[160px] resize-none text-lg leading-relaxed shadow-sm cursor-default"
                    placeholder="Describe how you want to transform the reference image..."
                  />
                   <div className="absolute top-2 right-2 flex gap-1 transition-all opacity-0 group-hover/prompt:opacity-100 [@media(hover:none)]:opacity-100">
                    {currentNodeId && generatedImages.length > 0 && (
                       <button 
                         type="button"
                         data-testid="view-prompt-btn"
                         onClick={() => handleOpenEditor({ readOnly: true })}
                         className="p-1.5 bg-white/80 hover:bg-white text-slate-500 hover:text-indigo-600 rounded-md shadow-sm border border-slate-200 backdrop-blur-sm transition-colors"
                         title="View Prompt Details"
                         disabled={isGenerating}
                       >
                         <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4" aria-hidden="true">
                             <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.644C3.412 9.143 6.82 6.5 11.25 6.5s7.838 2.643 9.214 5.178c.154.283.154.628 0 .911-1.376 2.535-4.782 5.178-9.214 5.178s-7.838-2.643-9.214-5.178ZM11.25 10.5a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3Z" />
                         </svg>
                       </button>
                     )}
                     {currentNodeId && generatedImages.length > 0 && (
                       <button 
                         type="button"
                         data-testid="fork-prompt-btn"
                         onClick={handleFork}
                         className="p-1.5 bg-white/80 hover:bg-white text-slate-500 hover:text-indigo-600 rounded-md shadow-sm border border-slate-200 backdrop-blur-sm transition-colors"
                         title="Fork Prompt"
                         disabled={isGenerating}
                       >
                           <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4" aria-hidden="true">
                               <circle cx="7.5" cy="5.25" r="2.25" />
                               <circle cx="7.5" cy="18.75" r="2.25" />
                               <circle cx="16.5" cy="5.25" r="2.25" />
                               <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 7.5v9M16.5 7.5v3a3 3 0 0 1-3 3h-6" />
                          </svg>
                       </button>
                     )}
                     {generatedImages.length === 0 && (
                     <button 
                       type="button"
                       data-testid="edit-prompt-btn"
                       onClick={() => handleOpenEditor()}
                       className="p-1.5 bg-white/80 hover:bg-white text-slate-500 hover:text-indigo-600 rounded-md shadow-sm border border-slate-200 backdrop-blur-sm transition-colors"
                       title="Open Prompt Editor"
                       disabled={isGenerating}
                     >
                       <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4" aria-hidden="true">
                           <path strokeLinecap="round" strokeLinejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                       </svg>
                     </button>
                     )}
                   </div>
                </div>
              </div>

              <div className="flex flex-col gap-4">
                <div className="grid grid-cols-2 gap-4">
                   <div className="flex flex-col gap-1">
                        <label htmlFor="image-size" className="text-xs font-semibold text-slate-500">Image Size</label>
                        <select 
                            id="image-size" 
                            className="bg-white border border-slate-200 rounded px-3 py-2 text-sm focus:ring-1 focus:ring-indigo-500 outline-none text-slate-900 shadow-sm"
                            value={imageSize}
                            onChange={(e) => setImageSize(e.target.value)}
                            disabled={isGenerating}
                        >
                            {imageSizes.map(s => <option key={s} value={s}>{s}</option>)}
                        </select>
                   </div>
                   
                   <div className="flex flex-col gap-1">
                        <label htmlFor="aspect-ratio" className="text-xs font-semibold text-slate-500">Aspect Ratio</label>
                        <select 
                            id="aspect-ratio" 
                            className="bg-white border border-slate-200 rounded px-3 py-2 text-sm focus:ring-1 focus:ring-indigo-500 outline-none text-slate-900 shadow-sm"
                            value={aspectRatio}
                            onChange={(e) => setAspectRatio(e.target.value)}
                            disabled={isGenerating}
                        >
                            {aspectRatios.map(r => <option key={r} value={r}>{r}</option>)}
                        </select>
                   </div>
                </div>
                
                <div className="pt-2">
                <button 
                  type="button"
                  data-testid="generate"
                  onClick={handleGenerate}
                  disabled={isGenerating}
                  className={`w-full py-4 px-6 rounded-lg text-lg tracking-wide ${generateButtonClass} ${isGenerating ? 'cursor-not-allowed text-slate-400' : ''}`}
                >
                  {isGenerating ? 'Generating...' : 'Generate'}
                </button>
                </div>
              </div>
            </div>
          </div>

          <div className="flex flex-col gap-3">
             <h2 className="text-xs font-bold uppercase tracking-wider text-slate-500">Results</h2>
             <div 
               data-testid="result-grid"
               className={`grid grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3 min-h-[300px] bg-slate-50 border-2 border-dashed border-slate-200 rounded-xl p-4
                 ${displayItems.length === 0 ? 'flex flex-col items-center justify-center' : ''}`}
             >
                {displayItems.length === 0 ? (
                  <>
                    <div className="text-5xl mb-3 opacity-30 grayscale">🖼️</div>
                    <p className="text-sm font-medium opacity-60 text-slate-500">No transformations generated yet</p>
                  </>
                ) : (
                  displayItems.map(img => (
                    <div 
                        key={img.id} 
                        className="relative group aspect-square"
                    >
                        <div
                            className="w-full h-full rounded-lg overflow-hidden bg-slate-100 shadow-md transition-all hover:scale-[1.02] cursor-zoom-in"
                            onClick={() => setPreviewImage(img)}
                        >
                          <img 
                            src={imageUrl(img)} 
                            alt="Generated" 
                            className="w-full h-full object-cover"
                            loading="lazy"
                          />
                        </div>
                        {img.resultId && (
                           <button
                             type="button"
                             data-testid={`result-delete-button-${img.resultId}`}
                             onClick={(e) => handleDeleteResult(img.resultId!, e)}
                             className="absolute top-2 right-2 p-1.5 bg-white/90 hover:bg-red-50 text-slate-400 hover:text-red-500 rounded-lg opacity-0 group-hover:opacity-100 transition-all shadow-sm z-10 backdrop-blur-sm"
                             title="Delete Result"
                           >
                             <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4" aria-hidden="true">
                               <path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                             </svg>
                           </button>
                        )}
                    </div>
                  ))
                )}
             </div>
          </div>

        </div>
      </div>
    </div>
  );
}
