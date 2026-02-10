import { useCallback, useEffect, useState } from 'react';
import { 
  ReactFlow, 
  Background, 
  Controls, 
  useNodesState, 
  useEdgesState,
  type Edge,
  type Node,
  ReactFlowProvider,
  MarkerType,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import { api, type PromptNode as APINode, type PromptMode } from '../lib/api';
import { PromptNode, type AppNode } from '../components/prompt-tree/PromptNode';
import { GraphControls } from '../components/prompt-tree/Controls';

const NODE_WIDTH = 280; // w-64 (256) + margin
const LAYER_HEIGHT = 200;

// Layout algorithm
type TreeLayoutNode = {
  id: string;
  children: TreeLayoutNode[];
  width: number;
  x: number;
  y: number;
  raw: APINode;
};

function buildHierarchy(nodes: APINode[], rootId: string): TreeLayoutNode | null {
  const map = new Map<string, TreeLayoutNode>();
  nodes.forEach(n => {
    map.set(n.id, { id: n.id, children: [], width: 0, x: 0, y: 0, raw: n });
  });

  let root: TreeLayoutNode | null = null;
  map.forEach(node => {
    if (node.id === rootId) {
      root = node;
    }
    if (node.raw.parent_id && map.has(node.raw.parent_id)) {
      map.get(node.raw.parent_id)!.children.push(node);
    }
  });
  
  // Sort children by created_at to keep consistent order
  map.forEach(node => {
    node.children.sort((a, b) => 
      new Date(a.raw.created_at).getTime() - new Date(b.raw.created_at).getTime()
    );
  });

  return root;
}

function calculateLayout(root: TreeLayoutNode, startX: number, startY: number): number {
  let xCursor = startX;
  
  function dfs(node: TreeLayoutNode, depth: number) {
    if (node.children.length === 0) {
      node.x = xCursor;
      xCursor += NODE_WIDTH;
    } else {
      node.children.forEach(child => dfs(child, depth + 1));
      node.x = (node.children[0].x + node.children[node.children.length - 1].x) / 2;
    }
    node.y = startY + depth * LAYER_HEIGHT;
  }

  dfs(root, 0);
  return xCursor; // Return next available X
}

const nodeTypes = {
  promptNode: PromptNode,
};

function PromptsPageContent() {
  const [nodes, setNodes, onNodesChange] = useNodesState<AppNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const [loading, setLoading] = useState(true);
  const [modeFilter, setModeFilter] = useState<PromptMode | 'all'>('all');
  const [showArchived, setShowArchived] = useState(false);
  
  const [selectedNodeId, setSelectedNodeId] = useState<string | undefined>();
  const [allApiNodes, setAllApiNodes] = useState<APINode[]>([]);

  // Actions
  const handleRename = async () => {
    if (!selectedNodeId) return;
    const node = allApiNodes.find(n => n.id === selectedNodeId);
    if (!node) return;
    
    const newTitle = window.prompt('请输入新标题', node.title);
    if (newTitle !== null && newTitle !== node.title) {
      try {
        await api.updatePrompt(selectedNodeId, { title: newTitle });
        // Optimistic update
        setAllApiNodes(prev => prev.map(n => n.id === selectedNodeId ? { ...n, title: newTitle } : n));
      } catch (err) {
        console.error(err);
        alert('重命名失败');
      }
    }
  };

  const handlePin = async () => {
    if (!selectedNodeId) return;
    const node = allApiNodes.find(n => n.id === selectedNodeId);
    if (!node) return;

    try {
      await api.updatePrompt(selectedNodeId, { pinned: !node.pinned });
      setAllApiNodes(prev => prev.map(n => n.id === selectedNodeId ? { ...n, pinned: !node.pinned } : n));
    } catch (err) {
      console.error(err);
      alert('操作失败');
    }
  };

  const handleArchive = async () => {
    if (!selectedNodeId) return;
    const node = allApiNodes.find(n => n.id === selectedNodeId);
    if (!node) return;

    const isArchived = !!node.archived_at;
    if (!isArchived && !window.confirm('确定要归档此节点吗？')) return;

    try {
      await api.updatePrompt(selectedNodeId, { archived: !isArchived });
      // Reload logic is simpler than optimistic update for archive time, but let's try strict update
      const now = new Date().toISOString();
      setAllApiNodes(prev => prev.map(n => n.id === selectedNodeId ? { ...n, archived_at: isArchived ? undefined : now } : n));
      if (!isArchived && !showArchived) {
        setSelectedNodeId(undefined); // Deselect if hiding
      }
    } catch (err) {
      console.error(err);
      alert('操作失败');
    }
  };

  const handleDelete = async () => {
    if (!selectedNodeId) return;
    if (!window.confirm('确定要永久删除此节点吗？该操作不可恢复。')) return;

    try {
      await api.deletePrompt(selectedNodeId);
      setAllApiNodes(prev => prev.filter(n => n.id !== selectedNodeId));
      setSelectedNodeId(undefined);
    } catch (err) {
      console.error(err);
      alert('删除失败');
    }
  };

  const fetchGraph = useCallback(async () => {
    setLoading(true);
    try {
      // 1. Fetch recent prompts
      const recent = await api.listPrompts({ 
        limit: 30, 
        include_archived: true, // Always fetch archived to build complete trees, filtering happens at render
        mode: modeFilter === 'all' ? undefined : modeFilter 
      });

      // 2. Fetch trees for unique root IDs (best effort guess: if no parent, it's root, or just fetch tree for each and let dedup handle it)
      // Actually, we can just fetch tree for *every* recent node to ensure we get their context
      // But we should dedup requests.
      // Optimization: if we have node A and B, and A.parent = B, fetching tree(B) includes A.
      // But we don't know the full relationship yet.
      // Let's just fetch all trees for the recent 30 items.
      
      const treePromises = recent.map(n => api.getPromptTree(n.id).catch(() => null));
      const trees = await Promise.all(treePromises);
      
      const allNodesMap = new Map<string, APINode>();
      const rootIds = new Set<string>();

      trees.forEach(tree => {
        if (!tree) return;
        rootIds.add(tree.root_id);
        tree.nodes.forEach(node => {
          allNodesMap.set(node.id, node);
        });
      });

      setAllApiNodes(Array.from(allNodesMap.values()));

    } catch (err) {
      console.error("Failed to load prompts graph", err);
    } finally {
      setLoading(false);
    }
  }, [modeFilter]);

  // Initial load
  useEffect(() => {
    fetchGraph();
  }, [fetchGraph]);

  // Re-calculate layout and nodes when data or filters change
  useEffect(() => {
    // 1. Filter nodes
    let filteredNodes = allApiNodes;
    if (modeFilter !== 'all') {
      filteredNodes = filteredNodes.filter(n => n.mode === modeFilter);
    }
    if (!showArchived) {
      filteredNodes = filteredNodes.filter(n => !n.archived_at);
    }

    // 2. Group by root (Forest reconstruction)
    // Since filtering might break trees, we need to be careful.
    // Approach: Build full trees first, then hide nodes? 
    // If we hide a parent, the children become floating?
    // Requirement: "关闭时图谱中不渲染 archived 节点".
    // If a middle node is archived, its children become disconnected visually if we just filter.
    // For now, let's just filter them out. If the tree breaks, so be it (or we can show edges to hidden parent?)
    // Let's stick to simple filtering.
    
    // We need to identify roots *within the filtered set*.
    const nodeMap = new Map(filteredNodes.map(n => [n.id, n]));
    const roots: APINode[] = [];
    
    filteredNodes.forEach(n => {
      // If parent is not in the filtered set, treat as root for layout purposes
      if (!n.parent_id || !nodeMap.has(n.parent_id)) {
        roots.push(n);
      }
    });

    // 3. Layout
    const flowNodes: AppNode[] = [];
    const flowEdges: Edge[] = [];
    
    let currentX = 0;
    
    // Sort roots by updated_at (desc)
    roots.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());

    roots.forEach(root => {
      // Re-build hierarchy for this root using ONLY filtered nodes
      // This ensures visual continuity within the subset
      const layoutRoot = buildHierarchy(filteredNodes, root.id);
      if (layoutRoot) {
        const nextX = calculateLayout(layoutRoot, currentX, 0);
        
        // Flatten to React Flow nodes
        const traverse = (n: TreeLayoutNode) => {
          flowNodes.push({
            id: n.id,
            type: 'promptNode',
            position: { x: n.x, y: n.y },
            data: { 
              node: n.raw,
              isSelected: n.id === selectedNodeId,
            },
          });
          
          n.raw.parent_id && nodeMap.has(n.raw.parent_id) && flowEdges.push({
            id: `${n.raw.parent_id}-${n.id}`,
            source: n.raw.parent_id,
            target: n.id,
            type: 'smoothstep',
            markerEnd: { type: MarkerType.ArrowClosed },
            style: { stroke: '#cbd5e1' }
          });

          n.children.forEach(traverse);
        };
        traverse(layoutRoot);

        currentX = nextX + 100; // Gap between trees
      }
    });

    setNodes(flowNodes);
    setEdges(flowEdges);

  }, [allApiNodes, modeFilter, showArchived, selectedNodeId, setNodes, setEdges]);

  // Selection handler
  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedNodeId(node.id);
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedNodeId(undefined);
  }, []);

  const selectedNode = allApiNodes.find(n => n.id === selectedNodeId);

  return (
    <div className="h-full w-full relative bg-slate-50">
      <GraphControls 
        mode={modeFilter} 
        setMode={setModeFilter}
        showArchived={showArchived}
        setShowArchived={setShowArchived}
        selectedNodeId={selectedNodeId}
        onRename={handleRename}
        onPin={handlePin}
        onArchive={handleArchive}
        onDelete={handleDelete}
        isPinned={selectedNode?.pinned}
        isArchived={!!selectedNode?.archived_at}
      />
      
      {loading && allApiNodes.length === 0 && (
        <div className="absolute inset-0 flex items-center justify-center z-50 bg-white/50">
          Loading...
        </div>
      )}

      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        nodeTypes={nodeTypes}
        fitView
        minZoom={0.1}
        maxZoom={2}
      >
        <Background color="#e2e8f0" gap={20} />
        <Controls />
      </ReactFlow>
    </div>
  );
}

export function PromptsPage() {
  return (
    <ReactFlowProvider>
      <PromptsPageContent />
    </ReactFlowProvider>
  );
}
