import React from 'react';
import type { Image } from '../lib/api';
import { imageUrl } from '../lib/imageUrl';

export interface Filters {
  query: string;
  date: 'today' | 'yesterday' | 'older' | 'all';
  type: 'all' | 'txt2img' | 'img2img';
}

export interface LibraryModalProps {
  open: boolean;
  title?: string;
  images: Image[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onCancel: () => void;
  onConfirm: () => void;
  filters: {
    query: string;
    date: 'today' | 'yesterday' | 'older' | 'all';
    type: 'all' | 'txt2img' | 'img2img';
  };
  onChangeFilters: (next: Filters) => void;
  loading?: boolean;
  onLoadMore?: () => void;
  hasMore?: boolean;
}

export function LibraryModal({
  open,
  title = 'Library',
  images,
  selectedId,
  onSelect,
  onCancel,
  onConfirm,
  filters,
  onChangeFilters,
  loading = false,
  onLoadMore,
  hasMore = false,
}: LibraryModalProps) {
  const lastLoadTime = React.useRef(0);

  if (!open) return null;

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    if (loading || !hasMore || !onLoadMore) return;

    const { scrollTop, scrollHeight, clientHeight } = e.currentTarget;
    if (scrollHeight - scrollTop - clientHeight < 200) {
      const now = Date.now();
      if (now - lastLoadTime.current > 500) {
        lastLoadTime.current = now;
        onLoadMore();
      }
    }
  };

  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChangeFilters({ ...filters, query: e.target.value });
  };

  const handleDateChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    onChangeFilters({ ...filters, date: e.target.value as Filters['date'] });
  };

  const handleTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    onChangeFilters({ ...filters, type: e.target.value as Filters['type'] });
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4"
      role="dialog"
      aria-modal="true"
    >
      <div className="w-full max-w-5xl h-[80vh] bg-white border border-slate-200 rounded-lg flex flex-col shadow-2xl overflow-hidden focus:outline-none ring-1 ring-black/5">
        <div className="h-14 border-b border-slate-200 px-4 flex items-center justify-between bg-white">
          <h2 className="text-lg font-medium text-slate-800">{title}</h2>
          <button
            onClick={onCancel}
            data-testid="library-close"
            className="p-2 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-md transition-colors"
            aria-label="Close library"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div className="p-4 border-b border-slate-200 flex flex-col sm:flex-row gap-4 bg-slate-50/50">
          <div className="flex-1">
            <input
              type="text"
              value={filters.query}
              onChange={handleSearchChange}
              placeholder="Search prompts..."
              className="w-full h-10 px-3 bg-white border border-slate-300 rounded-md text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all shadow-sm"
              data-testid="library-search"
            />
          </div>
          <div className="flex gap-4">
            <select
              value={filters.date}
              onChange={handleDateChange}
              className="h-10 px-3 bg-white border border-slate-300 rounded-md text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all min-w-[120px] shadow-sm"
              data-testid="library-date"
            >
              <option value="all">All Dates</option>
              <option value="today">Today</option>
              <option value="yesterday">Yesterday</option>
              <option value="older">Older</option>
            </select>
            <select
              value={filters.type}
              onChange={handleTypeChange}
              className="h-10 px-3 bg-white border border-slate-300 rounded-md text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all min-w-[120px] shadow-sm"
              data-testid="library-type"
            >
              <option value="all">All Types</option>
              <option value="txt2img">Txt2Img</option>
              <option value="img2img">Img2Img</option>
            </select>
          </div>
        </div>

        <div 
          className="flex-1 overflow-y-auto p-4 bg-slate-50"
          data-testid="library-grid"
          onScroll={handleScroll}
        >
          {loading && images.length === 0 ? (
            <div className="flex items-center justify-center h-full text-slate-500">
              <span className="animate-pulse">Loading library...</span>
            </div>
          ) : images.length === 0 ? (
            <div className="flex items-center justify-center h-full text-slate-500">
              No images found matching your filters.
            </div>
          ) : (
            <>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
                {images.map((img) => {
                  const isSelected = selectedId === img.id;
                  return (
                    <button
                      key={img.id}
                      onClick={() => onSelect(img.id)}
                      className={`
                        group relative aspect-square rounded-lg overflow-hidden border transition-all
                        ${isSelected 
                          ? 'border-blue-500 ring-2 ring-blue-500/50 shadow-lg shadow-blue-500/10' 
                          : 'border-slate-200 hover:border-slate-300 hover:shadow-md'
                        }
                      `}
                    >
                      <img
                        src={imageUrl(img)}
                        alt={`Generated ${img.created_at}`}
                        className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
                        loading="lazy"
                      />
                      <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity p-2 flex flex-col justify-end">
                        <span className="text-xs text-white/90 truncate font-mono">
                          {img.width}x{img.height}
                        </span>
                      </div>
                      {isSelected && (
                        <div className="absolute top-2 right-2 bg-blue-500 text-white p-1 rounded-full shadow-lg">
                          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round">
                            <polyline points="20 6 9 17 4 12"></polyline>
                          </svg>
                        </div>
                      )}
                    </button>
                  );
                })}
              </div>
              {loading && (
                <div className="py-4 text-center text-slate-500 text-sm animate-pulse w-full">
                  Loading more...
                </div>
              )}
            </>
          )}
        </div>

        <div className="p-4 border-t border-slate-200 bg-white flex justify-end gap-3">
          <button
            onClick={onCancel}
            data-testid="library-cancel"
            className="px-4 py-2 text-sm font-medium text-slate-600 hover:text-slate-800 hover:bg-slate-100 rounded-md transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={!selectedId}
            data-testid="library-confirm"
            className={`
              px-4 py-2 text-sm font-medium rounded-md transition-all shadow-sm border
              ${selectedId 
                ? 'bg-white text-slate-900 border-slate-200 hover:bg-slate-50' 
                : 'bg-slate-50 text-slate-400 border-slate-200 cursor-not-allowed'
              }
            `}
          >
            Confirm Selection
          </button>
        </div>
      </div>
    </div>
  );
}
