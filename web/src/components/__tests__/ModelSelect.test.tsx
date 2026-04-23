import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ModelSelect } from '../ModelSelect'
import type { ModelInfo } from '../../lib/api'

const makeModel = (id: string, label: string): ModelInfo => ({
  id,
  label,
  allowed_sizes: ['1K', '2K'],
  allowed_aspect_ratios: ['1:1', '3:2'],
  default_size: '1K',
})

describe('ModelSelect', () => {
  it('renders an option per enabled model', () => {
    const models = [makeModel('gpt-image-2', 'GPT Image 2'), makeModel('gemini-3-pro-image', 'Gemini 3 Pro')]

    render(<ModelSelect value="gpt-image-2" onChange={vi.fn()} models={models} />)

    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(2)
    expect(options[0]).toHaveTextContent('GPT Image 2')
    expect(options[1]).toHaveTextContent('Gemini 3 Pro')
  })

  it('onChange fires with the new model id', () => {
    const onChange = vi.fn()
    const models = [makeModel('gpt-image-2', 'GPT Image 2'), makeModel('gemini-3-pro-image', 'Gemini 3 Pro')]

    render(<ModelSelect value="gpt-image-2" onChange={onChange} models={models} />)

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'gemini-3-pro-image' } })

    expect(onChange).toHaveBeenCalledWith('gemini-3-pro-image')
  })

  it('reflects the controlled value', () => {
    const models = [makeModel('gpt-image-2', 'GPT Image 2'), makeModel('gemini-3-pro-image', 'Gemini 3 Pro')]

    render(<ModelSelect value="gpt-image-2" onChange={vi.fn()} models={models} />)

    expect(screen.getByRole('combobox')).toHaveValue('gpt-image-2')
  })
})
