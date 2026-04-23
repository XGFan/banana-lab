export const imageSizes = ['1K', '2K', '4K'] as const
export type ImageSize = (typeof imageSizes)[number]

export const aspectRatios = [
  '1:1',
  '2:3',
  '3:2',
  '3:4',
  '4:3',
  '4:5',
  '5:4',
  '9:16',
  '16:9',
  '21:9',
] as const
export type AspectRatio = (typeof aspectRatios)[number]

const gemini3ProImageData: Record<ImageSize, Record<AspectRatio, { width: number; height: number }>> = {
  '1K': {
    '1:1': { width: 1024, height: 1024 },
    '2:3': { width: 848, height: 1264 },
    '3:2': { width: 1264, height: 848 },
    '3:4': { width: 896, height: 1200 },
    '4:3': { width: 1200, height: 896 },
    '4:5': { width: 928, height: 1152 },
    '5:4': { width: 1152, height: 928 },
    '9:16': { width: 768, height: 1376 },
    '16:9': { width: 1376, height: 768 },
    '21:9': { width: 1584, height: 672 },
  },
  '2K': {
    '1:1': { width: 2048, height: 2048 },
    '2:3': { width: 1696, height: 2528 },
    '3:2': { width: 2528, height: 1696 },
    '3:4': { width: 1792, height: 2400 },
    '4:3': { width: 2400, height: 1792 },
    '4:5': { width: 1856, height: 2304 },
    '5:4': { width: 2304, height: 1856 },
    '9:16': { width: 1536, height: 2752 },
    '16:9': { width: 2752, height: 1536 },
    '21:9': { width: 3168, height: 1344 },
  },
  '4K': {
    '1:1': { width: 4096, height: 4096 },
    '2:3': { width: 3392, height: 5056 },
    '3:2': { width: 5056, height: 3392 },
    '3:4': { width: 3584, height: 4800 },
    '4:3': { width: 4800, height: 3584 },
    '4:5': { width: 3712, height: 4608 },
    '5:4': { width: 4608, height: 3712 },
    '9:16': { width: 3072, height: 5504 },
    '16:9': { width: 5504, height: 3072 },
    '21:9': { width: 6336, height: 2688 },
  },
}

// gpt-image-2 only supports 1K and 2K; max edge is 3840.
// 16:9 approximated as 1.75 ratio (1792/1024, 3584/2048) — multiples of 16.
// 4:3 approximated as 1.3125 ratio (1344/1024, 2688/2048) — multiples of 16.
// 4:5, 5:4, 21:9 not natively supported by gpt-image-2; fallback to nearest 3:2/2:3.
// 4K key included with capped values (3840 max edge) because TypeScript requires all ImageSize keys.
const gptImage2Data: Record<ImageSize, Record<AspectRatio, { width: number; height: number }>> = {
  '1K': {
    '1:1':  { width: 1024, height: 1024 },
    '3:2':  { width: 1536, height: 1024 },
    '2:3':  { width: 1024, height: 1536 },
    '16:9': { width: 1792, height: 1024 },
    '9:16': { width: 1024, height: 1792 },
    '4:3':  { width: 1344, height: 1024 },
    '3:4':  { width: 1024, height: 1344 },
    '4:5':  { width: 1024, height: 1280 }, // fallback: nearest supported
    '5:4':  { width: 1280, height: 1024 }, // fallback: nearest supported
    '21:9': { width: 1792, height: 768  }, // fallback: capped at 16:9 width
  },
  '2K': {
    '1:1':  { width: 2048, height: 2048 },
    '3:2':  { width: 3072, height: 2048 },
    '2:3':  { width: 2048, height: 3072 },
    '16:9': { width: 3584, height: 2048 },
    '9:16': { width: 2048, height: 3584 },
    '4:3':  { width: 2688, height: 2048 },
    '3:4':  { width: 2048, height: 2688 },
    '4:5':  { width: 2048, height: 2560 }, // fallback: nearest supported
    '5:4':  { width: 2560, height: 2048 }, // fallback: nearest supported
    '21:9': { width: 3584, height: 1536 }, // fallback: capped at max edge 3840
  },
  // gpt-image-2 max edge is 3840; 4K (4096) exceeds that limit.
  // These entries are unreachable at runtime (registry lists allowed_sizes: [1K, 2K] only).
  '4K': {
    '1:1':  { width: 3840, height: 3840 },
    '3:2':  { width: 3840, height: 2560 },
    '2:3':  { width: 2560, height: 3840 },
    '16:9': { width: 3840, height: 2160 },
    '9:16': { width: 2160, height: 3840 },
    '4:3':  { width: 3840, height: 2880 },
    '3:4':  { width: 2880, height: 3840 },
    '4:5':  { width: 3072, height: 3840 },
    '5:4':  { width: 3840, height: 3072 },
    '21:9': { width: 3840, height: 1648 },
  },
}

/**
 * Returns pixel dimensions for a given model, image size, and aspect ratio.
 * Falls back to gemini3ProImageData for unknown model IDs (legacy safety).
 * modelId is optional to preserve backward-compat with existing call sites.
 */
export const getDimensions = (modelIdOrSize: string, imageSizeOrRatio: string, maybeAspectRatio?: string) => {
  // Support both old 2-arg form getDimensions(size, ratio) and new 3-arg form getDimensions(modelId, size, ratio)
  let modelId: string
  let imageSize: string
  let aspectRatio: string

  if (maybeAspectRatio !== undefined) {
    modelId = modelIdOrSize
    imageSize = imageSizeOrRatio
    aspectRatio = maybeAspectRatio
  } else {
    // Legacy 2-arg form: first arg is imageSize, second is aspectRatio
    modelId = ''
    imageSize = modelIdOrSize
    aspectRatio = imageSizeOrRatio
  }

  const table = modelId === 'gpt-image-2' ? gptImage2Data : gemini3ProImageData

  const sizeEntry = table[imageSize as ImageSize]
  if (!sizeEntry) {
    throw new Error(`Unsupported image size: ${imageSize}`)
  }

  const dimensions = sizeEntry[aspectRatio as AspectRatio]
  if (!dimensions) {
    throw new Error(`Unsupported aspect ratio: ${aspectRatio}`)
  }

  return dimensions
}
