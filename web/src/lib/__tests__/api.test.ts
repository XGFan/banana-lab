import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, getModels } from '../api'

describe('prompt snippet api client', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists prompt snippets', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify([{ id: 's1', name: 'tone', content_fragments: [] }]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    const snippets = await api.listPromptSnippets()

    expect(fetchSpy).toHaveBeenCalledWith('/api/prompt-snippets', expect.any(Object))
    expect(snippets).toEqual([{ id: 's1', name: 'tone', content_fragments: [] }])
  })

  it('creates prompt snippet with snake_case fields and optional id', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ id: 's1', name: 'tone', content_fragments: [{ type: 'text', text: 'warm' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    await api.createPromptSnippet({
      id: 'snippet-1',
      name: 'tone',
      content_fragments: [{ type: 'text', text: 'warm' }],
    })

    expect(fetchSpy).toHaveBeenCalledWith(
      '/api/prompt-snippets',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          id: 'snippet-1',
          name: 'tone',
          content_fragments: [{ type: 'text', text: 'warm' }],
        }),
      })
    )
  })

  it('keeps create behavior unchanged when id is omitted', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ id: 's1', name: 'tone', content_fragments: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    await api.createPromptSnippet({
      name: 'tone',
      content_fragments: [],
    })

    expect(fetchSpy).toHaveBeenCalledWith(
      '/api/prompt-snippets',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          name: 'tone',
          content_fragments: [],
        }),
      })
    )
  })

  it('maps 404 error into ApiError.code', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'not_found' }), {
        status: 404,
        statusText: 'Not Found',
        headers: { 'Content-Type': 'application/json' },
      })
    )

    await expect(api.deletePromptSnippet('missing-id')).rejects.toMatchObject({
      status: 404,
      code: 'not_found',
    })
  })
})

describe('getModels', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches /api/models and returns the models array', async () => {
    const mockModels = [
      { id: 'gpt-image-2', label: 'GPT Image 2', allowed_sizes: ['1K', '2K'], allowed_aspect_ratios: ['1:1', '3:2'], default_size: '1K' },
      { id: 'gemini-3-pro-image', label: 'Gemini 3 Pro Image', allowed_sizes: ['1K', '2K', '4K'], allowed_aspect_ratios: ['1:1', '3:2', '21:9'], default_size: '1K' },
    ]

    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ models: mockModels }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    const result = await getModels()

    expect(result).toEqual(mockModels)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/models', expect.any(Object))
  })

  it('surfaces API errors when fetch returns error status', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'internal' }), {
        status: 500,
        statusText: 'Internal Server Error',
        headers: { 'Content-Type': 'application/json' },
      })
    )

    await expect(getModels()).rejects.toMatchObject({
      status: 500,
    })
  })
})
