import { useCallback, useEffect, useMemo, useRef, useState, type UIEvent } from 'react';
import { NavLink, useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { api, type PromptMode, type PromptNode } from '../lib/api';
import { SidebarItem } from './sidebar/SidebarItem';

const PAGE_SIZE = 30;
const SCROLL_THRESHOLD = 200;

type PromptCache = {
  items: PromptNode[];
  offset: number;
  loading: boolean;
  initialized: boolean;
  hasMore: boolean;
  error?: string;
};

const emptyCache: PromptCache = {
  items: [],
  offset: 0,
  loading: false,
  initialized: false,
  hasMore: true,
  error: undefined,
};

export function Sidebar() {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const activeNodeId = searchParams.get('node_id');
  const mode: PromptMode = location.pathname.startsWith('/img2img') ? 'img2img' : 'txt2img';

  const [promptsByMode, setPromptsByMode] = useState<Record<PromptMode, PromptCache>>({
    txt2img: { ...emptyCache },
    img2img: { ...emptyCache },
  });

  const promptsRef = useRef(promptsByMode);
  const loadingMapRef = useRef<Record<PromptMode, boolean>>({ txt2img: false, img2img: false });
  const scrollDebounceRef = useRef<number | null>(null);

  useEffect(() => {
    promptsRef.current = promptsByMode;
  }, [promptsByMode]);

  const loadPrompts = useCallback(async (targetMode: PromptMode) => {
    const current = promptsRef.current[targetMode];
    if (loadingMapRef.current[targetMode] || current.loading || !current.hasMore) {
      return;
    }

    loadingMapRef.current[targetMode] = true;
    setPromptsByMode((prev) => ({
      ...prev,
      [targetMode]: { ...prev[targetMode], loading: true, error: undefined },
    }));

    try {
      const data = await api.listPrompts({
        mode: targetMode,
        limit: PAGE_SIZE,
        offset: current.offset,
      });

      setPromptsByMode((prev) => {
        const prevItems = prev[targetMode].items;
        const existingIds = new Set(prevItems.map(n => n.id));
        const newItems = data.filter(n => !existingIds.has(n.id));
        
        const nextItems = [...prevItems, ...newItems];
        
        return {
          ...prev,
          [targetMode]: {
            ...prev[targetMode],
            items: nextItems,
            offset: nextItems.length,
            loading: false,
            initialized: true,
            hasMore: data.length === PAGE_SIZE,
          },
        };
      });
    } catch {
      setPromptsByMode((prev) => ({
        ...prev,
        [targetMode]: {
          ...prev[targetMode],
          loading: false,
          initialized: true,
          error: '加载失败',
        },
      }));
    } finally {
      loadingMapRef.current[targetMode] = false;
    }
  }, []);

  const reloadPrompts = useCallback((targetMode: PromptMode) => {
    setPromptsByMode((prev) => ({
      ...prev,
      [targetMode]: { ...emptyCache },
    }));
  }, []);

  const currentState = promptsByMode[mode];

  useEffect(() => {
    if (!currentState.initialized && !currentState.loading) {
      void loadPrompts(mode);
    }
  }, [currentState.initialized, currentState.loading, loadPrompts, mode]);

  useEffect(() => {
    if (activeNodeId && currentState.initialized && !currentState.items.find(n => n.id === activeNodeId)) {
      reloadPrompts(mode);
    }
  }, [activeNodeId, currentState.initialized, currentState.items, mode, reloadPrompts]);

  const handleScroll = useCallback((event: UIEvent<HTMLDivElement>) => {
    const target = event.currentTarget;
    if (scrollDebounceRef.current) {
      window.clearTimeout(scrollDebounceRef.current);
    }

    scrollDebounceRef.current = window.setTimeout(() => {
      const distanceToBottom = target.scrollHeight - target.scrollTop - target.clientHeight;
      if (distanceToBottom < SCROLL_THRESHOLD) {
        void loadPrompts(mode);
      }
    }, 120);
  }, [loadPrompts, mode]);

  useEffect(() => {
    return () => {
      if (scrollDebounceRef.current) {
        window.clearTimeout(scrollDebounceRef.current);
      }
    };
  }, []);

  const handleRename = useCallback(async (id: string, newTitle: string) => {
    setPromptsByMode((prev) => {
      const modeCache = prev[mode];
      const newItems = modeCache.items.map((item) => 
        item.id === id ? { ...item, title: newTitle } : item
      );
      return {
        ...prev,
        [mode]: { ...modeCache, items: newItems }
      };
    });

    try {
      await api.updatePrompt(id, { title: newTitle });
    } catch (error) {
      console.error('Rename failed', error);
      reloadPrompts(mode);
    }
  }, [mode, reloadPrompts]);

  const handleArchive = useCallback(async (id: string) => {
    if (!window.confirm('确定要归档这个节点吗？它将从列表中移除。')) {
      return;
    }

    setPromptsByMode((prev) => {
      const modeCache = prev[mode];
      const newItems = modeCache.items.filter((item) => item.id !== id);
      return {
        ...prev,
        [mode]: { ...modeCache, items: newItems }
      };
    });

    if (activeNodeId === id) {
      navigate(`/${mode}`);
    }

    try {
      await api.updatePrompt(id, { archived: true });
    } catch (error) {
      console.error('Archive failed', error);
      reloadPrompts(mode);
    }
  }, [mode, activeNodeId, navigate, reloadPrompts]);

  const handlePin = useCallback(async (id: string, pinned: boolean) => {
    try {
      await api.updatePrompt(id, { pinned });
      reloadPrompts(mode);
    } catch (error) {
      console.error('Pin failed', error);
    }
  }, [mode, reloadPrompts]);

  const listContent = useMemo(() => {
    if (!currentState.initialized && currentState.loading) {
      return <div className="mt-8 text-center text-sm text-slate-400">加载中…</div>;
    }

    if (currentState.initialized && !currentState.loading && currentState.items.length === 0) {
      return <div className="mt-8 text-center text-sm text-slate-400">暂无节点</div>;
    }

    return (
      <div className="space-y-0.5 pb-20">
        {currentState.items.map((promptNode) => {
          const isActive = promptNode.id === activeNodeId;
          return (
            <SidebarItem
              key={promptNode.id}
              node={promptNode}
              isActive={isActive}
              onNavigate={(id) => {
                const params = new URLSearchParams({ node_id: id });
                navigate(`/${promptNode.mode}?${params.toString()}`);
              }}
              onRename={handleRename}
              onArchive={handleArchive}
              onPin={handlePin}
            />
          );
        })}
        {currentState.loading && currentState.items.length > 0 ? (
          <div className="py-3 text-center text-xs text-slate-400">加载中…</div>
        ) : null}
        {currentState.error ? (
          <div className="py-2 text-center text-xs text-rose-500">{currentState.error}</div>
        ) : null}
      </div>
    );
  }, [activeNodeId, currentState, handleArchive, handlePin, handleRename, navigate]);

  return (
    <nav className="h-screen w-64 flex flex-col bg-slate-50 border-r border-slate-200 font-sans">
      <div className="h-16 flex items-center px-5 shrink-0" data-testid="sidebar-logo">
        <h1 className="text-xl font-bold tracking-tight text-slate-900">
          Banana Lab<span className="text-yellow-500">.</span>
        </h1>
      </div>

      <div className="px-3 shrink-0 select-none">
        <div className="w-full bg-slate-100 p-1 rounded-lg grid grid-cols-2 gap-1">
          <TabLink to="/txt2img" testId="tab-txt2img" icon={<TextIcon className="w-4 h-4" />} label="文生图" />
          <TabLink to="/img2img" testId="tab-img2img" icon={<ImageIcon className="w-4 h-4" />} label="图文生图" />
        </div>
      </div>

      <div className="p-3 shrink-0">
        <button
          type="button"
          data-testid="new-node-btn"
          onClick={() => navigate(`/${mode}?action=new`)}
          className="w-full flex items-center justify-center gap-2 bg-white text-slate-900 border border-slate-200 hover:bg-slate-50 text-sm font-medium py-2.5 px-4 rounded-lg transition-colors shadow-sm active:scale-[0.98]"
        >
          <PlusIcon className="w-4 h-4" />
          新建节点
        </button>
      </div>

      <div className="px-4 py-2 flex items-center justify-between shrink-0">
        <span className="text-xs font-semibold text-slate-500 uppercase tracking-wider">最近节点</span>
        <div className="flex gap-3">
          <NavLink
            to="/archive"
            data-testid="archive-link"
            className="text-xs font-medium text-slate-500 hover:text-slate-800 transition-colors"
          >
            归档
          </NavLink>
          <NavLink
            to="/prompts"
            data-testid="manage-prompts-btn"
            className="text-xs font-medium text-slate-500 hover:text-slate-800 transition-colors"
          >
            管理
          </NavLink>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-3 pb-4" data-testid="prompts-list" onScroll={handleScroll}>
        {listContent}
      </div>
    </nav>
  );
}

function TabLink({ to, testId, icon, label }: { to: string; testId: string; icon: React.ReactNode; label: string }) {
  return (
    <NavLink
      to={to}
      data-testid={testId}
      className={({ isActive }) =>
        `w-full flex items-center justify-center gap-2 py-2 text-sm font-medium rounded-md transition-all ${
          isActive
            ? 'bg-white text-slate-900 shadow-sm'
            : 'text-slate-500 hover:text-slate-900 hover:bg-white/60'
        }`
      }
    >
      {({ isActive }) => (
        <>
          <span className={isActive ? 'text-slate-900' : 'text-slate-400'}>{icon}</span>
          {label}
        </>
      )}
    </NavLink>
  );
}

function TextIcon({ className }: { className?: string }) {
  return (
    <svg className={className} xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <title>Text Icon</title>
      <path d="M17 6.1H3" />
      <path d="M21 12.1H3" />
      <path d="M15.1 18H3" />
    </svg>
  );
}

function ImageIcon({ className }: { className?: string }) {
  return (
    <svg className={className} xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <title>Image Icon</title>
      <rect width="18" height="18" x="3" y="3" rx="2" ry="2" />
      <circle cx="9" cy="9" r="2" />
      <path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21" />
    </svg>
  );
}

function PlusIcon({ className }: { className?: string }) {
  return (
    <svg className={className} xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <title>Plus Icon</title>
      <path d="M12 5v14" />
      <path d="M5 12h14" />
    </svg>
  );
}
