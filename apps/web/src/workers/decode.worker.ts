import jsQR from 'jsqr';

export interface DecodeWorkerMessage {
  id: string;
  data: Uint8ClampedArray;
  width: number;
  height: number;
  expectedPayload?: string;
}

export interface DecodeWorkerResponse {
  id: string;
  success: boolean;
  decoded?: string;
  error?: string;
}

self.onmessage = (e: MessageEvent<DecodeWorkerMessage>) => {
  const { id, data, width, height, expectedPayload } = e.data;

  try {
    const code = jsQR(data, width, height);

    if (!code) {
      const resp: DecodeWorkerResponse = {
        id,
        success: false,
        error: 'QR code unreadable by scanner',
      };
      self.postMessage(resp);
      return;
    }

    const matches = expectedPayload ? code.data === expectedPayload : true;
    const resp: DecodeWorkerResponse = {
      id,
      success: matches,
      decoded: code.data,
      error: matches ? undefined : 'Decoded data did not match expected payload',
    };
    self.postMessage(resp);
  } catch (err: any) {
    const resp: DecodeWorkerResponse = {
      id,
      success: false,
      error: err?.message || 'Decode processing error',
    };
    self.postMessage(resp);
  }
};
