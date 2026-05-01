"use client";

import { useState, useRef } from "react";
import { cn } from "@/lib/utils";

interface LogoUploadProps {
    onLogoChange: (logoDataUrl: string | null) => void;
    className?: string;
}

export function LogoUpload({ onLogoChange, className }: LogoUploadProps) {
    const [preview, setPreview] = useState<string | null>(null);
    const [isDragging, setIsDragging] = useState(false);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const handleFile = (file: File) => {
        if (!file.type.startsWith("image/")) {
            alert("Please upload an image file");
            return;
        }

        if (file.size > 2 * 1024 * 1024) {
            alert("File size must be less than 2MB");
            return;
        }

        const reader = new FileReader();
        reader.onload = (e) => {
            const dataUrl = e.target?.result as string;
            setPreview(dataUrl);
            onLogoChange(dataUrl);
        };
        reader.readAsDataURL(file);
    };

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragging(false);
        const file = e.dataTransfer.files[0];
        if (file) handleFile(file);
    };

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragging(true);
    };

    const handleDragLeave = () => {
        setIsDragging(false);
    };

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (file) handleFile(file);
    };

    const clearLogo = () => {
        setPreview(null);
        onLogoChange(null);
        if (fileInputRef.current) {
            fileInputRef.current.value = "";
        }
    };

    return (
        <div className={cn("relative", className)}>
            <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                onChange={handleInputChange}
                className="hidden"
                id="logo-upload"
            />

            {preview ? (
                <div className="flex items-center gap-3 p-3 bg-gray-800 rounded-lg border border-gray-700">
                    <img src={preview} alt="Logo preview" className="w-10 h-10 object-contain rounded" />
                    <div className="flex-1 min-w-0">
                        <p className="text-sm text-white truncate">Logo added</p>
                        <p className="text-xs text-gray-400">Click to change</p>
                    </div>
                    <button
                        onClick={clearLogo}
                        className="p-1.5 text-gray-400 hover:text-red-400 hover:bg-gray-700 rounded transition-colors"
                        title="Remove logo"
                    >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>
            ) : (
                <label
                    htmlFor="logo-upload"
                    onDrop={handleDrop}
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    className={cn(
                        "flex items-center gap-3 p-3 rounded-lg border border-dashed cursor-pointer transition-all",
                        isDragging
                            ? "border-purple-500 bg-purple-500/10"
                            : "border-white/10 hover:border-purple-500/50 bg-slate-800/50 hover:bg-slate-800"
                    )}
                >
                    <div className="w-10 h-10 rounded-lg bg-gray-700 flex items-center justify-center">
                        <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                        </svg>
                    </div>
                    <div className="flex-1">
                        <p className="text-sm text-white">Add logo to center</p>
                        <p className="text-xs text-gray-400">PNG, JPG up to 2MB</p>
                    </div>
                </label>
            )}
        </div>
    );
}
