import { describe, expect, it } from 'vitest'
import { buildImg2ImgPayload, buildTxt2ImgPayload } from '../generatePayload'

const fragmentsState = {
  groupOrder: ['group-1'],
  groups: {
    'group-1': {
      id: 'group-1',
      name: 'Main',
      promptIds: ['prompt-1'],
      collapsed: false,
    },
  },
  prompts: {
    'prompt-1': {
      id: 'prompt-1',
      content: 'hello',
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    },
  },
}

describe('buildTxt2ImgPayload', () => {
  it('builds payload with dimensions and empty reference images', () => {
    const payload = buildTxt2ImgPayload({
      parentNodeId: 'node-1',
      prompt: 'hello',
      aspectRatio: '16:9',
    })

    expect(payload).toMatchObject({
      mode: 'txt2img',
      parent_node_id: 'node-1',
      prompt: 'hello',
      aspect_ratio: '16:9',
      reference_image_ids: [],
      width: 1376,
      height: 768,
    })
    expect('source_image_id' in payload).toBe(false)
  })

  it('normalizes optional fields', () => {
    const payload = buildTxt2ImgPayload({
      prompt: 'hello',
      negativePrompt: '   ',
      cfgScaleText: '7.5',
      stepsText: '20',
      aspectRatio: '1:1',
    })

    expect(payload.negative_prompt).toBeUndefined()
    expect(payload.cfg_scale).toBe(7.5)
    expect(payload.steps).toBe(20)
  })

  it('passes through fragments when provided', () => {
    const payload = buildTxt2ImgPayload({
      prompt: 'hello',
      aspectRatio: '1:1',
      fragments: fragmentsState,
    })

    expect(payload.fragments).toEqual(fragmentsState)
    expect(payload.prompt).toBe('hello')
  })
})

describe('buildImg2ImgPayload', () => {
  it('builds payload with reference image ids and defaults', () => {
    const payload = buildImg2ImgPayload({
      parentNodeId: 'node-parent',
      prompt: 'transform',
      referenceImageIds: ['img-1', 'img-2'],
    })

    expect(payload).toMatchObject({
      mode: 'img2img',
      parent_node_id: 'node-parent',
      prompt: 'transform',
      aspect_ratio: '1:1',
      reference_image_ids: ['img-1', 'img-2'],
      width: 1024,
      height: 1024,
    })
  })

  it('passes through fragments for img2img payload', () => {
    const payload = buildImg2ImgPayload({
      prompt: 'transform',
      referenceImageIds: ['img-1'],
      fragments: fragmentsState,
    })

    expect(payload.fragments).toEqual(fragmentsState)
    expect(payload.prompt).toBe('transform')
  })
})

describe('buildTxt2ImgPayload model routing', () => {
  it('carries gpt-image-2 model through and uses its preset dims', () => {
    const payload = buildTxt2ImgPayload({
      model: 'gpt-image-2',
      prompt: 'x',
      aspectRatio: '1:1',
      imageSize: '1K',
    })

    expect(payload.model).toBe('gpt-image-2')
    expect(payload.width).toBe(1024)
    expect(payload.height).toBe(1024)
  })

  it('uses gpt-image-2 3:2 dims (diverges from gemini)', () => {
    const payload = buildTxt2ImgPayload({
      model: 'gpt-image-2',
      prompt: 'x',
      aspectRatio: '3:2',
      imageSize: '1K',
    })

    expect(payload.model).toBe('gpt-image-2')
    // gpt-image-2 1K 3:2 → 1536x1024; gemini would be 1264x848
    expect(payload.width).toBe(1536)
    expect(payload.height).toBe(1024)
  })

  it('uses gemini dims when model=gemini-3-pro-image', () => {
    const payload = buildTxt2ImgPayload({
      model: 'gemini-3-pro-image',
      prompt: 'x',
      aspectRatio: '3:2',
      imageSize: '1K',
    })

    expect(payload.model).toBe('gemini-3-pro-image')
    expect(payload.width).toBe(1264)
    expect(payload.height).toBe(848)
  })
})

describe('buildImg2ImgPayload model routing', () => {
  it('carries gpt-image-2 model through and uses its preset dims', () => {
    const payload = buildImg2ImgPayload({
      model: 'gpt-image-2',
      prompt: 'x',
      referenceImageIds: ['img-1'],
      aspectRatio: '3:2',
      imageSize: '1K',
    })

    expect(payload.model).toBe('gpt-image-2')
    expect(payload.width).toBe(1536)
    expect(payload.height).toBe(1024)
  })
})
