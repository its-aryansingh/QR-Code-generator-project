import React, { useState, useEffect } from 'react';
import { X, Lock, Download, FileImage, FileCode, FileText } from 'lucide-react';
import { useAuthStore } from '@/lib/auth';
import QRCodeStyling from 'qr-code-styling';
import jsPDF from 'jspdf';
import Link from 'next/link';

interface DownloadModalProps {
  isOpen: boolean;
  onClose: () => void;
  qrImage: string | null;
  selectedType: string;
  qrContent: string;
}

export const DownloadModal: React.FC<DownloadModalProps> = ({
  isOpen,
  onClose,
  qrImage,
  selectedType,
  qrContent
}) => {
  const { isAuthenticated } = useAuthStore();
  const isPro = isAuthenticated; // Currently checking if authenticated

  const [format, setFormat] = useState<'png' | 'svg' | 'pdf'>('png');
  const [size, setSize] = useState<number>(300);
  const [downloading, setDownloading] = useState(false);

  useEffect(() => {
    if (isOpen) {
      setFormat('png');
      setSize(300);
    }
  }, [isOpen]);

  // Handle escape key
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) onClose();
    };
    window.addEventListener('keydown', handleEsc);
    return () => window.removeEventListener('keydown', handleEsc);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const handleDownload = async () => {
    if (!qrImage && format === 'png') return;
    setDownloading(true);

    try {
      const fileName = `qrit-${selectedType}-${Date.now()}`;

      if (format === 'png') {
        const canvas = document.createElement('canvas');
        canvas.width = size;
        canvas.height = size;
        const ctx = canvas.getContext('2d');
        if (ctx && qrImage) {
          const img = new Image();
          await new Promise((resolve, reject) => {
            img.onload = resolve;
            img.onerror = reject;
            img.src = qrImage;
          });
          ctx.drawImage(img, 0, 0, size, size);

          const a = document.createElement('a');
          a.href = canvas.toDataURL('image/png');
          a.download = `${fileName}.png`;
          a.click();
        }
      } else if (format === 'svg') {
        const qrCode = new QRCodeStyling({
          width: size,
          height: size,
          data: qrContent,
          margin: 10,
          qrOptions: { typeNumber: 0, mode: "Byte", errorCorrectionLevel: "Q" },
          imageOptions: { hideBackgroundDots: true, imageSize: 0.4, margin: 0 }
        });
        await qrCode.download({ name: fileName, extension: 'svg' });
      } else if (format === 'pdf') {
        const pdf = new jsPDF('p', 'pt', [size, size]);
        if (qrImage) {
          pdf.addImage(qrImage, 'PNG', 0, 0, size, size);
          pdf.save(`${fileName}.pdf`);
        }
      }
    } catch (err) {
      console.error('Download failed:', err);
    } finally {
      setDownloading(false);
      onClose();
    }
  };

  const isLocked = (f: string) => (f === 'svg' || f === 'pdf') && !isPro;
  const isSizeLocked = size > 500 && !isPro;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-200" onClick={onClose}>
      <div
        className="bg-zinc-900 border border-zinc-800 rounded-2xl w-full max-w-md shadow-2xl overflow-hidden animate-in zoom-in-95 duration-200"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between p-4 border-b border-zinc-800">
          <h2 className="text-lg font-bold text-white">Download QR Code</h2>
          <button onClick={onClose} className="p-1 rounded-lg hover:bg-zinc-800 text-zinc-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="p-6 space-y-6">
          <div className="flex justify-center">
            <div className="w-40 h-40 bg-white rounded-xl flex items-center justify-center p-2 border border-zinc-700">
              {qrImage ? <img src={qrImage} alt="QR Preview" className="w-full h-full object-contain" /> : <div className="w-full h-full bg-zinc-200 rounded animate-pulse" />}
            </div>
          </div>

          <div className="space-y-3">
            <label className="text-sm font-medium text-zinc-300">Format</label>
            <div className="grid grid-cols-3 gap-3">
              <button
                onClick={() => setFormat('png')}
                className={`flex flex-col items-center gap-2 p-3 rounded-xl border transition-all ${format === 'png' ? 'bg-violet-600/20 border-violet-500 text-violet-300' : 'bg-zinc-800/50 border-zinc-700 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-300'}`}
              >
                <FileImage className="w-6 h-6" />
                <span className="text-xs font-semibold">PNG</span>
              </button>

              <button
                onClick={() => setFormat('svg')}
                className={`relative flex flex-col items-center gap-2 p-3 rounded-xl border transition-all ${format === 'svg' ? 'bg-violet-600/20 border-violet-500 text-violet-300' : 'bg-zinc-800/50 border-zinc-700 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-300'}`}
              >
                {!isPro && <Lock className="absolute top-2 right-2 w-3 h-3 text-violet-400" />}
                <FileCode className="w-6 h-6" />
                <span className="text-xs font-semibold">SVG</span>
              </button>

              <button
                onClick={() => setFormat('pdf')}
                className={`relative flex flex-col items-center gap-2 p-3 rounded-xl border transition-all ${format === 'pdf' ? 'bg-violet-600/20 border-violet-500 text-violet-300' : 'bg-zinc-800/50 border-zinc-700 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-300'}`}
              >
                {!isPro && <Lock className="absolute top-2 right-2 w-3 h-3 text-violet-400" />}
                <FileText className="w-6 h-6" />
                <span className="text-xs font-semibold">PDF</span>
              </button>
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium text-zinc-300">Image Size</label>
              <span className="text-xs font-mono text-zinc-400">{size}x{size} px</span>
            </div>
            <div className="relative">
              <input
                type="range"
                min="300"
                max="2000"
                step="100"
                value={size}
                onChange={e => setSize(Number(e.target.value))}
                className="w-full h-2 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-violet-500"
              />
              {!isPro && size > 500 && (
                <div className="absolute -top-6 right-0 bg-violet-600 text-white text-[10px] px-2 py-0.5 rounded flex items-center gap-1">
                  <Lock className="w-3 h-3" /> Pro size
                </div>
              )}
            </div>
            <div className="flex justify-between text-xs text-zinc-500">
              <span>300px</span>
              <span>2000px</span>
            </div>
          </div>
        </div>

        <div className="p-4 border-t border-zinc-800 bg-zinc-900/50">
          {(isLocked(format) || isSizeLocked) ? (
            <Link href="/register" className="w-full flex items-center justify-center gap-2 py-3 rounded-xl bg-violet-600 hover:bg-violet-700 text-white text-sm font-medium transition-all shadow-lg shadow-violet-600/20">
              <Lock className="w-4 h-4" /> Upgrade to Pro to Download
            </Link>
          ) : (
            <button
              onClick={handleDownload}
              disabled={downloading || (!qrImage && format === 'png')}
              className="w-full flex items-center justify-center gap-2 py-3 rounded-xl bg-white text-zinc-900 hover:bg-zinc-200 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium transition-all"
            >
              {downloading ? (
                <div className="w-4 h-4 border-2 border-zinc-900 border-t-transparent rounded-full animate-spin" />
              ) : (
                <Download className="w-4 h-4" />
              )}
              {downloading ? 'Preparing...' : `Download ${format.toUpperCase()}`}
            </button>
          )}
        </div>
      </div>
    </div>
  );
};
