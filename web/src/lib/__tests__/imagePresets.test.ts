import { describe, expect, it } from 'vitest'
import { getDimensions } from '../imagePresets'

const gemini3ProImageData = {
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
} as const

describe('getDimensions', () => {
  it('returns matching width/height for all presets', () => {
    Object.entries(gemini3ProImageData).forEach(([imageSize, ratios]) => {
      Object.entries(ratios).forEach(([aspectRatio, expected]) => {
        expect(getDimensions(imageSize, aspectRatio)).toEqual(expected)
      })
    })
  })

  it('throws on unsupported image size', () => {
    expect(() => getDimensions('8K', '1:1')).toThrow('Unsupported image size')
  })

  it('throws on unsupported aspect ratio', () => {
    expect(() => getDimensions('1K', '10:1')).toThrow('Unsupported aspect ratio')
  })
})
