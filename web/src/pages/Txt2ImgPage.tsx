import { useState, useEffect, useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import type { ApiError, Image, Result, PromptFragment, PromptSnippet } from '../lib/api';
import { imageUrl } from '../lib/imageUrl';
import { buildTxt2ImgPayload } from '../lib/generatePayload';
import { imageSizes, aspectRatios } from '../lib/imagePresets';
import { ImagePreviewModal } from '../components/ImagePreviewModal';
import { generateButtonClass } from '../lib/buttonStyles';
import { PromptEditorSDK, type PromptEditorValue, type PromptEditorSDKHandle, type PromptEditorLibrary } from 'prompt-editor-sdk';
import 'prompt-editor-sdk/style.css';
import { toSdkValue, collectionToPlainPrompt, collectionFromPlainPrompt, snippetsFromLibrary, libraryFromSnippets } from '../lib/promptEditor/adapter';
import type { PromptCollectionState } from '../lib/promptEditor/types';

export function Txt2ImgPage() {
  const [currentNodeId, setCurrentNodeId] = useState<string | undefined>(undefined);

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [sdkValue, setSdkValue] = useState<PromptEditorValue | null>(null);
  const [sdkLibrary, setSdkLibrary] = useState<PromptEditorLibrary>(libraryFromSnippets([]));
  const sdkRef = useRef<PromptEditorSDKHandle>(null);
  const librarySnippetsRef = useRef<PromptSnippet[]>([]);
  const librarySyncQueueRef = useRef<Promise<void>>(Promise.resolve());

  const [prompt, setPrompt] = useState('');
  const [fragments, setFragments] = useState<PromptCollectionState | PromptFragment[]>([]);
  const [baselinePrompt, setBaselinePrompt] = useState('');
  const [negativePrompt, setNegativePrompt] = useState('');
  const [cfgScaleText, setCfgScaleText] = useState('');
  const [stepsText, setStepsText] = useState('');
  const [aspectRatio, setAspectRatio] = useState('1:1');
  const [imageSize, setImageSize] = useState('1K');
  const isDirty = prompt.trim() !== baselinePrompt.trim();
  
  const [rerollCount, setRerollCount] = useState('1');
  const [isLooping, setIsLooping] = useState(false);
  const [loopProgress, setLoopProgress] = useState({ current: 0, total: 0 });
  const stopLoopRef = useRef(false);

  const [images, setImages] = useState<Image[]>([]);
  const [results, setResults] = useState<Result[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [previewImage, setPreviewImage] = useState<Image | null>(null);
  const [showDeleteNodeModal, setShowDeleteNodeModal] = useState(false);

  const [searchParams, setSearchParams] = useSearchParams();

  const isEmpty = !prompt.trim();

  const clearSelectedNode = useCallback(() => {
    setCurrentNodeId(undefined);
    setPrompt('');
    setFragments([]);
    setBaselinePrompt('');
    setAspectRatio('1:1');
    setImageSize('1K');
    setImages([]);
    setResults([]);
  }, []);

  useEffect(() => {
    const nid = searchParams.get('node_id');

    if (nid) {
      setLoading(true);
      api.getPromptTree(nid)
        .then(() => api.getNode(nid))
        .then(({ node, images, results }) => {
          const restoredAspectRatio = node.aspect_ratio ?? '1:1';
          const restoredImageSize = node.image_size ?? '1K';
          setCurrentNodeId(node.id);
          setPrompt(node.prompt);
          setFragments(node.fragments || []);
          setBaselinePrompt(node.prompt);
          setAspectRatio(restoredAspectRatio);
          setImageSize(restoredImageSize);
          setImages(images);
          setResults(results || []);
        })
        .catch((err: unknown) => {
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
          setError('Failed to restore node');
        })
        .finally(() => {
          setLoading(false);
        });
    } else {
      clearSelectedNode();
    }
  }, [clearSelectedNode, searchParams, setSearchParams]);

  const handleGenerate = useCallback(async (isReroll = false) => {
    if (loading || isLooping) return;
    if (isReroll && !currentNodeId) {
      setError('请先生成一个节点');
      return;
    }
    if (!prompt.trim()) {
      setError('请输入提示词');
      return;
    }
    
    setError(null);
    stopLoopRef.current = false;

    // Calculate count
    let count = 1;
    if (isReroll) {
      const parsed = parseInt(rerollCount, 10);
      count = isNaN(parsed) || parsed < 1 ? 1 : parsed;
    }

    if (count > 1) {
      setIsLooping(true);
      setLoopProgress({ current: 0, total: count });
    } else {
      setLoading(true);
    }
    
    try {
      let lastNodeId: string | undefined;

      for (let i = 0; i < count; i++) {
        if (stopLoopRef.current) break;
        
        if (count > 1) setLoopProgress(p => ({ ...p, current: i + 1 }));

        const res = isReroll
          ? await api.reroll(currentNodeId || '', {
              aspect_ratio: aspectRatio,
              image_size: imageSize,
            })
          : await api.generate(buildTxt2ImgPayload({
              parentNodeId: currentNodeId,
              prompt,
              fragments,
              negativePrompt,
              cfgScaleText,
              stepsText,
              aspectRatio,
              imageSize,
            }));

        setImages(res.images);
        setResults(res.results || []);
        setCurrentNodeId(res.node.id);
        setBaselinePrompt(prompt);
        setFragments(res.node.fragments || []);
        lastNodeId = res.node.id;
      }

      if (lastNodeId) {
        setSearchParams((prev) => {
          const nextParams = new URLSearchParams(prev);
          nextParams.set('node_id', lastNodeId);
          return nextParams;
        }, { replace: true });
      }

    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "系统繁忙，请稍后重试");
    } finally {
      setLoading(false);
      setIsLooping(false);
      setLoopProgress({ current: 0, total: 0 });
    }
  }, [currentNodeId, prompt, negativePrompt, cfgScaleText, stepsText, aspectRatio, imageSize, loading, isLooping, rerollCount, setSearchParams]);

  const handleStop = () => {
    stopLoopRef.current = true;
  };

  const handleRerollCountBlur = () => {
    const parsed = parseInt(rerollCount, 10);
    if (isNaN(parsed) || parsed < 1) {
        setRerollCount('1');
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

  const [saveError, setSaveError] = useState<string | null>(null);

  const handleOpenEditor = async () => {
    try {
      const snippets = await api.listPromptSnippets();
      librarySnippetsRef.current = snippets;
      const initialValue = toSdkValue(prompt, fragments, snippets);
      setSdkValue(initialValue.value);
      setSdkLibrary(initialValue.library);
      setSaveError(null);
      setIsModalOpen(true);
    } catch (err) {
      console.error('Failed to load library:', err);
      setError('Failed to load snippet library');
      // Fallback: open without library
      const initialValue = toSdkValue(prompt, fragments, []);
      librarySnippetsRef.current = [];
      setSdkValue(initialValue.value);
      setSdkLibrary(initialValue.library);
      setSaveError(null);
      setIsModalOpen(true);
    }
  };

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

  const handleSdkSave = (finalFragments: PromptCollectionState) => {
      const nextCollection = finalFragments;
      const nextPrompt = collectionToPlainPrompt(nextCollection);
      
      setPrompt(nextPrompt);
      setFragments(nextCollection);
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

  const displayItems = results.length > 0 
    ? results.map(r => {
        const img = images.find(i => i.id === r.asset_id);
        return img ? { ...img, resultId: r.id } : null;
      }).filter((item): item is Image & { resultId: string } => item !== null)
    : images.map(img => ({ ...img, resultId: undefined }));

  return (
    <div className="flex flex-col h-full w-full bg-slate-50 text-slate-900 font-sans overflow-hidden">
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
              />
             <div className="p-4 border-t border-slate-200 flex justify-between items-center gap-2 bg-slate-50">
                <div className="flex-1 text-sm text-red-600 font-medium px-2">
                    {saveError && <span data-testid="sdk-save-error">{saveError}</span>}
                </div>
                <div className="flex gap-2">
                    <button
                        onClick={() => setIsModalOpen(false)}
                        className="px-4 py-2 text-sm font-medium text-slate-600 hover:text-slate-800 hover:bg-slate-100 rounded-lg transition-colors"
                    >
                        Cancel
                    </button>
                    <button
                        data-testid="sdk-save-btn"
                        onClick={handleSaveClick}
                        className="px-4 py-2 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg transition-colors shadow-sm"
                    >
                        Save Changes
                    </button>
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

      <div className="shrink-0 px-6 py-3 border-b border-slate-200 flex items-center space-x-2 text-sm text-slate-500 bg-white z-20" data-testid="breadcrumbs">
        <span className="hover:text-slate-700 cursor-pointer transition-colors">Home</span>
        <span>/</span>
        <span className="text-slate-700 font-medium">Text to Image</span>
      </div>

      <div className="flex-1 flex flex-col min-h-0">
          
        <div className="shrink-0 p-6 bg-white border-b border-slate-200 shadow-sm z-10 overflow-y-auto max-h-[50vh]">
             <div className="max-w-6xl mx-auto grid grid-cols-2 gap-8">
                 <div className="flex flex-col gap-2">
                    <div className="flex justify-between items-center">
                        <div className="flex items-center gap-2">
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
                    </div>
                    <div className="relative group/prompt flex-1 flex flex-col">
                        <textarea 
                            id="prompt-input"
                            data-testid="prompt"
                            className="flex-1 w-full border border-slate-200 rounded-lg p-4 text-slate-900 placeholder-slate-400 focus:outline-none min-h-[160px] resize-none text-base shadow-inner bg-slate-100 cursor-default"
                            placeholder="Prompt will appear here..."
                            value={prompt}
                            readOnly
                            disabled={loading || isLooping}
                        />
                        <button 
                            type="button"
                            data-testid="edit-prompt-btn"
                            onClick={handleOpenEditor}
                            className="absolute top-2 right-2 p-1.5 bg-white/80 hover:bg-white text-slate-500 hover:text-indigo-600 rounded-md shadow-sm border border-slate-200 transition-all opacity-0 group-hover/prompt:opacity-100 [@media(hover:none)]:opacity-100 backdrop-blur-sm"
                            title="Open Prompt Editor"
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4" aria-hidden="true">
                                <path strokeLinecap="round" strokeLinejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                            </svg>
                        </button>
                    </div>
                 </div>

                 <div className="flex flex-col justify-between gap-6">
                     <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                        <div className="flex flex-col gap-1">
                            <label htmlFor="image-size" className="text-xs font-semibold text-slate-500">Image Size</label>
                            <select 
                                id="image-size" 
                                className="bg-white border border-slate-200 rounded px-3 py-2 text-sm focus:ring-1 focus:ring-indigo-500 outline-none text-slate-900 shadow-sm"
                                value={imageSize}
                                onChange={(e) => setImageSize(e.target.value)}
                                disabled={loading || isLooping}
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
                                disabled={loading || isLooping}
                            >
                                {aspectRatios.map(r => <option key={r} value={r}>{r}</option>)}
                            </select>
                        </div>

                        <div className="flex flex-col gap-1">
                            <label htmlFor="cfg-scale" className="text-xs font-semibold text-slate-500">CFG Scale</label>
                            <input 
                                type="number" 
                                id="cfg-scale" 
                                className="bg-white border border-slate-200 rounded px-3 py-2 text-sm focus:ring-1 focus:ring-indigo-500 outline-none placeholder-slate-400 text-slate-900 shadow-sm"
                                placeholder="7.0"
                                value={cfgScaleText}
                                onChange={(e) => setCfgScaleText(e.target.value)}
                                disabled={loading || isLooping}
                            />
                        </div>

                        <div className="flex flex-col gap-1">
                            <label htmlFor="steps" className="text-xs font-semibold text-slate-500">Steps</label>
                            <input 
                                type="number" 
                                id="steps" 
                                className="bg-white border border-slate-200 rounded px-3 py-2 text-sm focus:ring-1 focus:ring-indigo-500 outline-none placeholder-slate-400 text-slate-900 shadow-sm"
                                placeholder="20"
                                value={stepsText}
                                onChange={(e) => setStepsText(e.target.value)}
                                disabled={loading || isLooping}
                            />
                        </div>
                        
                        <div className="col-span-full flex flex-col gap-1">
                            <label htmlFor="negative-prompt" className="text-xs font-semibold text-slate-500">Negative Prompt</label>
                            <input 
                                type="text" 
                                id="negative-prompt" 
                                className="bg-white border border-slate-200 rounded px-3 py-2 text-sm focus:ring-1 focus:ring-indigo-500 outline-none placeholder-slate-400 text-slate-900 shadow-sm"
                                placeholder="Items to remove..."
                                value={negativePrompt}
                                onChange={(e) => setNegativePrompt(e.target.value)}
                                disabled={loading || isLooping}
                            />
                        </div>
                     </div>

                     <div className="flex flex-col gap-2">
                        <div className="flex flex-row gap-3">
                            {isDirty ? (
                                <button 
                                    type="button"
                                    data-testid="generate"
                                    className={`flex-1 py-3 px-6 rounded-lg ${generateButtonClass}`}
                                    onClick={() => handleGenerate(false)}
                                    disabled={loading || isLooping || isEmpty}
                                >
                                    {loading && !isLooping ? 'Generating...' : 'Generate & Fork'}
                                </button>
                            ) : (
                                <div className="flex-1 flex gap-2">
                                    {isLooping ? (
                                        <button 
                                            type="button"
                                            data-testid="stop-reroll"
                                            className="flex-1 bg-red-50 hover:bg-red-100 text-red-600 border border-red-200 font-medium rounded-lg transition-colors cursor-pointer shadow-sm"
                                            onClick={handleStop}
                                        >
                                            Stop
                                        </button>
                                    ) : (
                                        <button 
                                            type="button"
                                             data-testid="reroll"
                                             className="flex-1 bg-white hover:bg-slate-50 text-slate-700 font-medium rounded-lg border border-slate-200 transition-colors cursor-pointer shadow-sm disabled:opacity-50 disabled:cursor-not-allowed"
                                             onClick={() => handleGenerate(true)}
                                             disabled={loading || !currentNodeId || isEmpty}
                                         >

                                             Re-roll
                                         </button>
                                    )}
                                    <input 
                                        type="number"
                                        data-testid="reroll-count"
                                        className="w-16 bg-white border border-slate-200 rounded-lg px-2 text-center text-sm font-medium focus:ring-1 focus:ring-indigo-500 outline-none shadow-sm"
                                        min="1"
                                        max="100"
                                        value={rerollCount}
                                        onChange={(e) => setRerollCount(e.target.value)}
                                        onBlur={handleRerollCountBlur}
                                        disabled={loading || isLooping}
                                        title="Batch count"
                                    />
                                </div>
                            )}
                        </div>
                        {isLooping && (
                            <div className="text-center text-xs font-mono text-indigo-600 font-medium animate-pulse">
                                Processing: {loopProgress.current} / {loopProgress.total}
                            </div>
                        )}
                        {error && (
                            <div data-testid="generate-error" className="text-sm text-red-600 text-center">
                                {error}
                            </div>
                        )}
                     </div>
                 </div>
             </div>
        </div>

        <div className="flex-1 overflow-y-auto p-6 bg-slate-50">
            <div className="max-w-6xl mx-auto flex flex-col gap-2">
                <h2 className="text-xs font-bold uppercase tracking-wider text-slate-500 mb-2">Results</h2>
                <div 
                data-testid="result-grid"
                className="grid grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3 min-h-[300px]"
                >
                    {displayItems.length > 0 ? (
                        displayItems.map((img) => (
                        <div key={img.id} className="relative aspect-square group">
                            <button
                                type="button"
                                className="w-full h-full rounded-xl overflow-hidden bg-white shadow-md cursor-zoom-in border border-slate-100 transition-all hover:shadow-lg hover:border-slate-300 hover:scale-[1.01]"
                                onClick={() => setPreviewImage(img)}
                            >
                                <img 
                                    src={imageUrl(img)} 
                                    alt={img.id} 
                                    className="w-full h-full object-cover"
                                    loading="lazy"
                                />
                                <div className="absolute inset-0 bg-black/0 group-hover:bg-black/10 transition-colors" />
                            </button>
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
                    ) : (
                        <div className="col-span-full flex flex-col items-center justify-center text-slate-400 h-full min-h-[300px] border-2 border-dashed border-slate-200 rounded-xl bg-white/50">
                        <div className="text-5xl mb-3 opacity-30 grayscale">🖼️</div>
                        <p className="text-sm font-medium text-slate-500 opacity-70">No images generated yet</p>
                        <p className="text-xs mt-1 opacity-50">Enter a prompt and hit generate to see magic happen</p>
                        </div>
                    )}
                </div>
            </div>
        </div>
        
      </div>
    </div>
  );
}
