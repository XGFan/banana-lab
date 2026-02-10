import { describe, expect, it } from 'vitest'
import { runReroll } from '../reroll'

describe('runReroll', () => {
  it('runs sequentially and reports progress', async () => {
    const order: number[] = []
    let active = 0
    let maxActive = 0
    const progress: Array<{ done: number; total: number }> = []

    const results = await runReroll({
      total: 3,
      onProgress: (done, total) => progress.push({ done, total }),
      runOnce: async (index) => {
        active += 1
        maxActive = Math.max(maxActive, active)
        await new Promise(resolve => setTimeout(resolve, 0))
        order.push(index)
        active -= 1
        return index
      },
    })

    expect(results).toEqual([0, 1, 2])
    expect(order).toEqual([0, 1, 2])
    expect(maxActive).toBe(1)
    expect(progress).toEqual([
      { done: 1, total: 3 },
      { done: 2, total: 3 },
      { done: 3, total: 3 },
    ])
  })

  it('stops early when shouldStop returns true', async () => {
    let shouldStop = false

    const results = await runReroll({
      total: 5,
      shouldStop: () => shouldStop,
      runOnce: async (index) => {
        if (index === 1) {
          shouldStop = true
        }
        return index
      },
    })

    expect(results).toEqual([0, 1])
  })
})
