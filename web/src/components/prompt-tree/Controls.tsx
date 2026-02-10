import type { PromptMode } from '../../lib/api';

interface ControlsProps {
  mode: PromptMode | 'all';
  setMode: (mode: PromptMode | 'all') => void;
  showArchived: boolean;
  setShowArchived: (show: boolean) => void;
  selectedNodeId?: string;
  onRename?: () => void;
  onPin?: () => void;
  onArchive?: () => void;
  onDelete?: () => void;
  isPinned?: boolean;
  isArchived?: boolean;
}

export function GraphControls({ 
  mode, setMode, 
  showArchived, setShowArchived,
  selectedNodeId,
  onRename, onPin, onArchive, onDelete,
  isPinned, isArchived
}: ControlsProps) {
  return (
    <div className="absolute top-4 left-4 z-10 flex flex-col gap-2">
      <div className="flex items-center gap-2 bg-white p-1.5 rounded-lg border border-slate-200 shadow-sm">
        <select 
          value={mode} 
          onChange={(e) => setMode(e.target.value as any)}
          className="text-sm border-none bg-transparent focus:ring-0 text-slate-700 font-medium cursor-pointer"
        >
          <option value="all">全部模式</option>
          <option value="txt2img">文生图</option>
          <option value="img2img">图生图</option>
        </select>
        <div className="w-px h-4 bg-slate-200 mx-1" />
        <label className="flex items-center gap-1.5 text-sm text-slate-600 px-2 cursor-pointer select-none">
          <input 
            type="checkbox" 
            checked={showArchived} 
            onChange={(e) => setShowArchived(e.target.checked)}
            className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
          />
          显示归档
        </label>
      </div>

      {selectedNodeId && (
        <div className="flex items-center gap-1 bg-white p-1.5 rounded-lg border border-slate-200 shadow-sm animate-in fade-in slide-in-from-top-2">
          <button onClick={onRename} className="px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50 rounded">
            重命名
          </button>
          <button onClick={onPin} className="px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50 rounded">
            {isPinned ? '取消置顶' : '置顶'}
          </button>
          <button onClick={onArchive} className="px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50 rounded">
            {isArchived ? '取消归档' : '归档'}
          </button>
          <div className="w-px h-4 bg-slate-200 mx-1" />
          <button onClick={onDelete} className="px-3 py-1.5 text-xs font-medium text-rose-600 hover:bg-rose-50 rounded">
            删除
          </button>
        </div>
      )}
    </div>
  );
}
