import { useEffect, useState } from 'react';
import type { Image } from '../lib/api';
import { imageUrl } from '../lib/imageUrl';

interface ImagePreviewModalProps {
  image: Image | null;
  onClose: () => void;
}

export function ImagePreviewModal({ image, onClose }: ImagePreviewModalProps) {
  const [imageDimensions, setImageDimensions] = useState<{ width: number; height: number } | null>(null);

  const formatSize = (dimensions: { width: number; height: number } | null): string => {
    if (!dimensions) return '--';
    return `${dimensions.width} × ${dimensions.height}`;
  };

  const gcd = (a: number, b: number): number => {
    let x = Math.abs(a);
    let y = Math.abs(b);
    while (y !== 0) {
      const t = y;
      y = x % y;
      x = t;
    }
    return x || 1;
  };

  const formatRatio = (dimensions: { width: number; height: number } | null): string => {
    if (!dimensions) return '--';
    const d = gcd(dimensions.width, dimensions.height);
    return `${dimensions.width / d}:${dimensions.height / d}`;
  };

  const formatGeneratedTime = (img: Image): string => {
    if (!img.created_at) return '--';
    const date = new Date(img.created_at);
    if (Number.isNaN(date.getTime())) return '--';
    return date.toLocaleString('zh-CN', { hour12: false });
  };

  useEffect(() => {
    if (image) {
      setImageDimensions(null);
      document.body.style.overflow = 'hidden';
      const handleEsc = (e: KeyboardEvent) => {
        if (e.key === 'Escape') onClose();
      };
      window.addEventListener('keydown', handleEsc);
      return () => {
        document.body.style.overflow = '';
        window.removeEventListener('keydown', handleEsc);
      };
    }
  }, [image, onClose]);

  if (!image) return null;

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/90 backdrop-blur-sm"
      onClick={onClose}
      data-testid="preview-modal-mask"
    >
      <div 
        className="relative max-w-[90vw] max-h-[90vh] flex flex-col items-center"
        onClick={e => e.stopPropagation()}
      >
        <img
          src={imageUrl(image)}
          alt="Preview"
          className="max-w-full max-h-[90vh] object-contain rounded-lg shadow-2xl"
          data-testid="preview-image"
          onLoad={e => {
            const width = e.currentTarget.naturalWidth;
            const height = e.currentTarget.naturalHeight;
            if (width > 0 && height > 0) {
              setImageDimensions({ width, height });
              return;
            }
            setImageDimensions(null);
          }}
          onError={() => setImageDimensions(null)}
        />
        <div className="absolute bottom-3 right-3 px-3 py-2 rounded-lg bg-black/55 backdrop-blur-sm text-white/95 text-xs leading-5 shadow-lg border border-white/10">
          <div>尺寸：{formatSize(imageDimensions)}</div>
          <div>比例：{formatRatio(imageDimensions)}</div>
          <div>生成时间：{formatGeneratedTime(image)}</div>
        </div>
        <button
            type="button"
            onClick={onClose}
            className="absolute -top-10 right-0 text-white/70 hover:text-white transition-colors bg-transparent border-0 p-1 hover:bg-white/10 rounded-full"
            aria-label="Close preview"
        >
             <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
               <line x1="18" y1="6" x2="6" y2="18"></line>
               <line x1="6" y1="6" x2="18" y2="18"></line>
             </svg>
        </button>
      </div>
    </div>
  );
}
