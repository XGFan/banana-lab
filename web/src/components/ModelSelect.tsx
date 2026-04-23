import type { ModelInfo } from '../lib/api'

interface ModelSelectProps {
  value: string
  onChange: (next: string) => void
  models: ModelInfo[]
}

export function ModelSelect({ value, onChange, models }: ModelSelectProps) {
  return (
    <select
      id="model-select"
      className="bg-white border border-slate-200 rounded px-3 py-2 text-sm focus:ring-1 focus:ring-indigo-500 outline-none text-slate-900 shadow-sm"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      {models.map((m) => (
        <option key={m.id} value={m.id}>
          {m.label}
        </option>
      ))}
    </select>
  )
}
