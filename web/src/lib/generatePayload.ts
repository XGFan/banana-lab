import { getDimensions } from './imagePresets'
import type { GenerateRequest, PromptCollectionState, PromptFragment } from './api'

type LegacyPromptFragments = PromptFragment[]

const normalizeFragments = (
  fragments?: PromptCollectionState | LegacyPromptFragments
): PromptCollectionState | undefined => {
  if (!fragments) return undefined
  if (!Array.isArray(fragments)) return fragments

  const groupId = 'legacy-group'
  const now = new Date().toISOString()
  const promptIds = fragments.map((_, idx) => `legacy-prompt-${idx}`)
  const prompts = Object.fromEntries(
    fragments.map((fragment, idx) => {
      const id = `legacy-prompt-${idx}`
      return [
        id,
        {
          id,
          content: fragment.text ?? '',
          createdAt: now,
          updatedAt: now,
        },
      ]
    })
  )

  return {
    groupOrder: [groupId],
    groups: {
      [groupId]: {
        id: groupId,
        name: 'Legacy',
        promptIds,
        collapsed: false,
      },
    },
    prompts,
  }
}

type Txt2ImgPayloadInput = {
  parentNodeId?: string
  prompt: string
  fragments?: PromptCollectionState | LegacyPromptFragments
  negativePrompt?: string
  cfgScaleText?: string
  stepsText?: string
  aspectRatio: string
  imageSize?: string
  model?: string
}

type Img2ImgPayloadInput = {
  parentNodeId?: string
  prompt: string
  fragments?: PromptCollectionState | LegacyPromptFragments
  referenceImageIds: string[]
  aspectRatio?: string
  imageSize?: string
  model?: string
}

export const buildTxt2ImgPayload = (input: Txt2ImgPayloadInput): GenerateRequest => {
  const normalizedNegativePrompt = input.negativePrompt?.trim()
  const cfgScaleTrimmed = input.cfgScaleText?.trim()
  const stepsTrimmed = input.stepsText?.trim()
  const imageSize = input.imageSize ?? '1K'
  const model = input.model ?? ''
  const { width, height } = getDimensions(model, imageSize, input.aspectRatio)

  return {
    mode: 'txt2img',
    parent_node_id: input.parentNodeId,
    prompt: input.prompt,
    fragments: normalizeFragments(input.fragments),
    negative_prompt: normalizedNegativePrompt ? normalizedNegativePrompt : undefined,
    cfg_scale: cfgScaleTrimmed ? Number(cfgScaleTrimmed) : undefined,
    steps: stepsTrimmed ? Number(stepsTrimmed) : undefined,
    aspect_ratio: input.aspectRatio,
    image_size: imageSize,
    width,
    height,
    reference_image_ids: [],
    model,
  }
}

export const buildImg2ImgPayload = (input: Img2ImgPayloadInput): GenerateRequest => {
  const imageSize = input.imageSize ?? '1K'
  const aspectRatio = input.aspectRatio ?? '1:1'
  const model = input.model ?? ''
  const { width, height } = getDimensions(model, imageSize, aspectRatio)

  return {
    mode: 'img2img',
    parent_node_id: input.parentNodeId,
    prompt: input.prompt,
    fragments: normalizeFragments(input.fragments),
    aspect_ratio: aspectRatio,
    image_size: imageSize,
    width,
    height,
    reference_image_ids: input.referenceImageIds,
    model,
  }
}
