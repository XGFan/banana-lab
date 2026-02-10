import { Handle, Position, type NodeProps, type Node } from '@xyflow/react';
import { useNavigate } from 'react-router-dom';
import type { PromptNode as APINode } from '../../lib/api';

export type PromptNodeData = {
  node: APINode;
  isSelected: boolean;
  // Actions are available via selection context in parent
  onPin?: (id: string, pinned: boolean) => void;
  onArchive?: (id: string, archived: boolean) => void;
  onDelete?: (id: string) => void;
  onRename?: (id: string, title: string) => void;
};

export type AppNode = Node<PromptNodeData, 'promptNode'>;

export function PromptNode({ data }: NodeProps<AppNode>) {
  const navigate = useNavigate();
  const { node } = data;
  const isArchived = !!node.archived_at;
  const isPinned = node.pinned;

  return (
    <div className={`
      relative w-64 rounded-lg border bg-white px-4 py-3 shadow-sm transition-all
      ${isArchived ? 'border-slate-200 bg-slate-50 opacity-75' : 'border-slate-300 hover:border-slate-400'}
      ${data.isSelected ? 'ring-2 ring-blue-500 border-blue-500' : ''}
    `}>
      <Handle type="target" position={Position.Top} className="!bg-slate-300" />
      
      <div className="flex items-start justify-between gap-2">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="font-medium text-slate-900 truncate text-sm" title={node.title || 'Untitled'}>
              {node.title || 'Untitled'}
            </h3>
            {isPinned && (
              <span className="text-xs text-yellow-600 bg-yellow-50 px-1.5 py-0.5 rounded">Pinned</span>
            )}
            {isArchived && (
              <span className="text-xs text-slate-500 bg-slate-100 px-1.5 py-0.5 rounded">Archived</span>
            )}
          </div>
          <p className="mt-1 text-xs text-slate-500 line-clamp-2 font-mono break-all" title={node.prompt}>
            {node.prompt}
          </p>
          <button
            type="button"
            className="mt-2 inline-flex items-center rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
            onClick={(e) => {
              e.stopPropagation();
              const params = new URLSearchParams({ node_id: node.id });
              navigate(`/${node.mode}?${params.toString()}`);
            }}
          >
            查看详情
          </button>
        </div>
      </div>

      <div className="mt-3 flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        {/* Actions are handled via context menu or separate controls in the main view usually, 
            but simple buttons here work for quick access if selected */}
      </div>

      <div className="absolute top-2 right-2 flex gap-1">
        {/* Status indicators */}
        <div className={`w-2 h-2 rounded-full ${node.mode === 'txt2img' ? 'bg-indigo-400' : 'bg-purple-400'}`} 
             title={node.mode} />
      </div>

      <Handle type="source" position={Position.Bottom} className="!bg-slate-300" />
    </div>
  );
}
