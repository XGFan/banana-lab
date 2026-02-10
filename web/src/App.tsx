import { Routes, Route, Navigate } from 'react-router-dom';
import { Sidebar } from './components/Sidebar';
import { Txt2ImgPage } from './pages/Txt2ImgPage';
import { Img2ImgPage } from './pages/Img2ImgPage';
import { PromptsPage } from './pages/PromptsPage';
import { ArchivePage } from './pages/ArchivePage';

function App() {
  return (
    <div className="flex h-screen w-screen overflow-hidden bg-white text-slate-900" data-testid="app-shell">
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
        <Routes>
          <Route path="/" element={<Navigate to="/txt2img" replace />} />
          <Route path="/txt2img" element={<Txt2ImgPage />} />
          <Route path="/img2img" element={<Img2ImgPage />} />
          <Route path="/prompts" element={<PromptsPage />} />
          <Route path="/archive" element={<ArchivePage />} />
          <Route path="*" element={<div className="p-8">404 - Not Found</div>} />
        </Routes>
      </main>
    </div>
  );
}

export default App;
