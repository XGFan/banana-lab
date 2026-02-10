import { useState, useEffect, useCallback } from 'react';
import { api, type ArchiveResponse } from '../lib/api';

export function ArchivePage() {
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [data, setData] = useState<ArchiveResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadData = useCallback(async (fromDate: string, toDate: string) => {
    if (fromDate && toDate && fromDate > toDate) {
      setError('开始时间不能晚于结束时间');
      return;
    }
    setError(null);
    setLoading(true);
    try {
      const result = await api.getArchive({
        from: fromDate ? new Date(fromDate).toISOString() : undefined,
        to: toDate ? new Date(toDate).toISOString() : undefined
      });
      setData(result);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData('', '');
  }, [loadData]);

  const handleApply = () => {
    loadData(from, to);
  };

  const isEmpty = data && data.txt2img.length === 0 && data.img2img.length === 0;

  return (
    <div className="p-8 max-w-7xl mx-auto" data-testid="archive-page">
      <h1 className="text-2xl font-bold mb-6 text-slate-900">全局归档库</h1>

      <div className="flex flex-wrap items-end gap-4 mb-8 bg-slate-50 p-4 rounded-lg border border-slate-200">
        <div className="flex-1 min-w-[200px]">
           <label htmlFor="from-date" className="block text-sm font-medium text-slate-700 mb-1">开始时间 (From)</label>
           <input
             id="from-date"
             type="datetime-local"
             className="block w-full rounded-md border-slate-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
             value={from}
             onChange={e => setFrom(e.target.value)}
             data-testid="archive-filter-from"
           />
        </div>
        <div className="flex-1 min-w-[200px]">
           <label htmlFor="to-date" className="block text-sm font-medium text-slate-700 mb-1">结束时间 (To)</label>
           <input
             id="to-date"
             type="datetime-local"
             className="block w-full rounded-md border-slate-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
             value={to}
             onChange={e => setTo(e.target.value)}
             data-testid="archive-filter-to"
           />
        </div>
        <button
          type="button"
          onClick={handleApply}
          className="bg-slate-900 text-white px-4 py-2 rounded-md hover:bg-slate-800 transition-colors text-sm font-medium shadow-sm"
          data-testid="archive-filter-apply"
        >
          筛选
        </button>
      </div>

      {error && (
        <div className="mb-6 p-4 bg-rose-50 border border-rose-200 text-rose-700 rounded-md text-sm">
          {error}
        </div>
      )}

      {loading && (
        <div className="text-center py-12">
          <p className="text-slate-500">加载中...</p>
        </div>
      )}

      {!loading && isEmpty && (
        <div data-testid="archive-empty" className="text-center py-20 bg-slate-50 rounded-lg border border-dashed border-slate-300">
          <p className="text-slate-500">暂无归档数据</p>
        </div>
      )}

      {!loading && data && (
        <div className="space-y-10">
          <section data-testid="archive-group-txt2img">
            <h2 className="text-lg font-semibold mb-4 text-slate-900 flex items-center gap-2">
              Txt2Img 
              <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-600 text-xs font-medium">{data.txt2img.length}</span>
            </h2>
            {data.txt2img.length > 0 ? (
              <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
                {data.txt2img.map(item => (
                  <div key={item.id} className="group relative bg-white border border-slate-200 rounded-lg overflow-hidden hover:shadow-md transition-shadow">
                      <div className="aspect-square bg-slate-100 flex items-center justify-center text-slate-400 text-xs break-all p-2">
                        <span className="opacity-50 group-hover:opacity-100 transition-opacity">{item.asset.file_path}</span>
                      </div>
                      <div className="p-2 bg-white border-t border-slate-100">
                        <div className="text-xs text-slate-500">{new Date(item.archived_at).toLocaleString()}</div>
                      </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-slate-400 italic">此分类下无数据</p>
            )}
          </section>

          <section data-testid="archive-group-img2img">
             <h2 className="text-lg font-semibold mb-4 text-slate-900 flex items-center gap-2">
              Img2Img
              <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-600 text-xs font-medium">{data.img2img.length}</span>
             </h2>
             {data.img2img.length > 0 ? (
              <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
                {data.img2img.map(item => (
                  <div key={item.id} className="group relative bg-white border border-slate-200 rounded-lg overflow-hidden hover:shadow-md transition-shadow">
                      <div className="aspect-square bg-slate-100 flex items-center justify-center text-slate-400 text-xs break-all p-2">
                         <span className="opacity-50 group-hover:opacity-100 transition-opacity">{item.asset.file_path}</span>
                      </div>
                      <div className="p-2 bg-white border-t border-slate-100">
                        <div className="text-xs text-slate-500">{new Date(item.archived_at).toLocaleString()}</div>
                      </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-slate-400 italic">此分类下无数据</p>
            )}
          </section>
        </div>
      )}
    </div>
  );
}
