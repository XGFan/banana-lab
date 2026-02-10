type RunRerollOptions<T> = {
  total: number
  runOnce: (index: number) => Promise<T>
  shouldStop?: () => boolean
  onProgress?: (done: number, total: number) => void
}

export const runReroll = async <T>({
  total,
  runOnce,
  shouldStop,
  onProgress,
}: RunRerollOptions<T>): Promise<T[]> => {
  const results: T[] = []

  for (let index = 0; index < total; index += 1) {
    if (shouldStop?.()) {
      break
    }

    const result = await runOnce(index)
    results.push(result)
    onProgress?.(results.length, total)

    if (shouldStop?.()) {
      break
    }
  }

  return results
}
