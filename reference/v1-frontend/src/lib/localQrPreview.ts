import QRCodeStyling from "qr-code-styling";

/** Browser-only PNG data URL; used when API preview fails (offline, CORS, rate limit, etc.). */
export async function getLocalQrDataUrl(data: string, size: number, options?: any): Promise<string> {
  const customOptions: any = {
    width: size,
    height: size,
    type: "canvas",
    data,
    margin: options?.quietZone !== undefined ? options.quietZone : 4,
    qrOptions: { errorCorrectionLevel: options?.errorLevel || "M" },
    dotsOptions: {
      type: options?.selectedDotStyle || "square",
      color: options?.qrColor || "#111111",
      ...(options?.useGradient && options?.gradientType === 'linear' ? {
        gradient: {
          type: "linear",
          rotation: (options.gradientRotation || 0) * Math.PI / 180,
          colorStops: [
            { offset: 0, color: options.gradientStart || "#000000" },
            { offset: 1, color: options.gradientEnd || "#000000" }
          ]
        }
      } : {}),
      ...(options?.useGradient && options?.gradientType === 'radial' ? {
        gradient: {
          type: "radial",
          colorStops: [
            { offset: 0, color: options.gradientStart || "#000000" },
            { offset: 1, color: options.gradientEnd || "#000000" }
          ]
        }
      } : {})
    },
    cornersSquareOptions: {
      type: options?.selectedCornerStyle === 'none' ? 'square' : (options?.selectedCornerStyle || "square"),
      color: options?.qrColor || "#111111"
    },
    cornersDotOptions: {
      type: options?.eyeInnerStyle || "square",
      color: options?.qrColor || "#111111"
    },
    backgroundOptions: {
      color: options?.transparentBg ? "transparent" : (options?.bgColor || "#ffffff")
    },
    imageOptions: {
      crossOrigin: "anonymous",
      margin: options?.logoMargin || 0,
      imageSize: options?.logoSize ? options.logoSize / 100 : 0.4
    }
  };

  if (options?.logoFile) {
    customOptions.image = options.logoFile;
  }

  const qr = new QRCodeStyling(customOptions);

  const blob = await qr.getRawData("png");
  if (!blob || !(blob instanceof Blob)) {
    throw new Error("QR preview blob missing");
  }
  return await new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onloadend = () => resolve(String(r.result));
    r.onerror = () => reject(r.error ?? new Error("read failed"));
    r.readAsDataURL(blob);
  });
}
