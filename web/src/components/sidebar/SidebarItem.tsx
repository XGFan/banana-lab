import { useState, useRef, useEffect } from 'react';
import { type PromptNode } from '../../lib/api';
import { SidebarMenu } from './SidebarMenu';

interface SidebarItemProps {
  node: PromptNode;
  isActive: boolean;
  onNavigate: (id: string) => void;
  onRename: (id: string, newTitle: string) => void;
  onArchive: (id: string) => void;
  onPin: (id: string, pinned: boolean) => void;
}

export function SidebarItem({ node, isActive, onNavigate, onRename, onArchive, onPin }: SidebarItemProps) {
  const [isHovering, setIsHovering] = useState(false);
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editTitle, setEditTitle] = useState(node.title);
  const inputRef = useRef<HTMLInputElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  const handleRenameSubmit = () => {
    if (editTitle.trim() && editTitle !== node.title) {
      onRename(node.id, editTitle);
    } else {
      setEditTitle(node.title);
    }
    setIsEditing(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleRenameSubmit();
    } else if (e.key === 'Escape') {
      setEditTitle(node.title);
      setIsEditing(false);
    }
  };

  return (
    <div
      className={`group relative flex items-center gap-2 px-3 py-2.5 rounded-lg text-sm transition-all cursor-pointer select-none mb-0.5 ${
        isActive 
          ? 'bg-yellow-50 text-slate-900 shadow-sm ring-1 ring-yellow-200/50' 
          : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
      }`}
      onMouseEnter={() => setIsHovering(true)}
      onMouseLeave={() => setIsHovering(false)}
      onClick={() => !isEditing && onNavigate(node.id)}
      data-testid={`sidebar-item-${node.id}`}
    >
      <div className={`shrink-0 transition-colors ${isActive ? 'text-yellow-600' : 'text-slate-400 group-hover:text-slate-500'}`}>
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
      </div>

      <div className="flex-1 min-w-0">
        {isEditing ? (
          <input
            ref={inputRef}
            type="text"
            value={editTitle}
            onChange={(e) => setEditTitle(e.target.value)}
            onBlur={handleRenameSubmit}
            onKeyDown={handleKeyDown}
            className="w-full bg-white border border-blue-500 rounded px-1.5 py-0.5 outline-none text-slate-900 text-sm shadow-sm"
            onClick={(e) => e.stopPropagation()}
          />
        ) : (
          <div className="truncate pr-6 relative font-medium leading-tight">
             {node.title || 'Untitled'}
          </div>
        )}
      </div>

      {/* Menu Button */}
      {(isHovering || isMenuOpen) && !isEditing && (
        <div className="absolute right-2 top-1/2 -translate-y-1/2 z-10 flex items-center bg-gradient-to-l from-white via-white/80 to-transparent pl-2">
           <button
              ref={menuButtonRef}
              className={`p-1 rounded-md transition-colors ${isMenuOpen ? 'bg-slate-200 text-slate-700' : 'hover:bg-slate-200 text-slate-500'}`}
              onClick={(e) => {
                e.stopPropagation();
                setIsMenuOpen(!isMenuOpen);
              }}
              aria-label="Options"
           >
             <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></svg>
           </button>
           
           <div className="absolute right-0 top-full mt-1">
             <SidebarMenu
                isOpen={isMenuOpen}
                onClose={() => setIsMenuOpen(false)}
                onRename={() => setIsEditing(true)}
                onArchive={() => onArchive(node.id)}
                onPin={() => onPin(node.id, !node.pinned)}
                isPinned={node.pinned}
             />
           </div>
        </div>
      )}

      {/* Pinned Indicator (only visible when not hovering/menu closed) */}
      {node.pinned && !isHovering && !isMenuOpen && (
        <div className="absolute right-3 top-1/2 -translate-y-1/2 text-yellow-500">
          <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="12" x2="12" y1="17" y2="22"/><path d="M5 17h14v-1.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1v4.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24Z"/></svg>
        </div>
      )}
    </div>
  );
}
