import { useState, useEffect, useRef } from 'react'
import { storage } from '@/lib/storage/adapter'
import type { PersistedTableState, DensityMode } from './types'

// v2: el default de densidad pasó de 'normal' a 'compact'. El estado por tabla se persiste, así
// que sin cambiar el prefijo toda tabla ya abierta seguiría leyendo su density:'normal' guardada y
// el default nuevo no se aplicaría jamás. Bumpear el prefijo descarta esos blobs viejos.
const PREFIX = 'data-table:v2:'
const DEBOUNCE_MS = 400

const DEFAULTS: PersistedTableState = {
  columnOrder: [],
  columnSizing: {},
  columnVisibility: {},
  columnPinning: { left: [], right: [] },
  sorting: [],
  density: 'compact',
  pageSize: 20,
}

export function useTableState(tableId: string, defaultDensity?: DensityMode, defaultPageSize?: number) {
  const defaults: PersistedTableState = {
    ...DEFAULTS,
    density: defaultDensity ?? DEFAULTS.density,
    pageSize: defaultPageSize ?? DEFAULTS.pageSize,
  }

  const [state, setState] = useState<PersistedTableState>(defaults)
  const [isLoaded, setIsLoaded] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const latestState = useRef(state)
  latestState.current = state

  useEffect(() => {
    storage.get<PersistedTableState>(`${PREFIX}${tableId}`)
      .then((saved) => {
        if (saved) setState({ ...defaults, ...saved })
      })
      .catch(() => {})
      .finally(() => setIsLoaded(true))
  }, [tableId])

  function persist(updates: Partial<PersistedTableState>) {
    setState((prev) => {
      const next = { ...prev, ...updates }
      if (timer.current) clearTimeout(timer.current)
      timer.current = setTimeout(() => {
        storage.set(`${PREFIX}${tableId}`, next).catch(() => {})
      }, DEBOUNCE_MS)
      return next
    })
  }

  function reset() {
    if (timer.current) clearTimeout(timer.current)
    setState(defaults)
    storage.delete(`${PREFIX}${tableId}`).catch(() => {})
  }

  return { state, persist, reset, isLoaded }
}
