export type FilterTarget = 'params' | 'body'

export interface FilterOption {
  label: string
  value: string
}

interface FilterBase {
  label: string
  target?: FilterTarget
  placeholder?: string
}

export type TextFilterDef<TFilters extends Record<string, unknown>> = FilterBase & {
  type: 'text'
  id: keyof TFilters & string
  /** Clase de ancho Tailwind para el input (default `w-36`). Ej: `'w-64'`, `'w-80'`. */
  width?: string
}

export type SelectFilterDef<TFilters extends Record<string, unknown>> = FilterBase & {
  type: 'select'
  id: keyof TFilters & string
  options: FilterOption[]
}

export type AsyncSelectFilterDef<TFilters extends Record<string, unknown>> = FilterBase & {
  type: 'asyncselect'
  id: keyof TFilters & string
  useOptions: () => { data?: FilterOption[]; isLoading?: boolean }
}

export type BooleanFilterDef<TFilters extends Record<string, unknown>> = FilterBase & {
  type: 'boolean'
  id: keyof TFilters & string
}

/** Date range controla dos keys de filtro, por eso `id` es un nombre lógico. */
export type DateRangeFilterDef<TFilters extends Record<string, unknown>> = FilterBase & {
  type: 'daterange'
  id: string
  fromKey: keyof TFilters & string
  toKey: keyof TFilters & string
}

export type FilterDef<TFilters extends Record<string, unknown>> =
  | TextFilterDef<TFilters>
  | SelectFilterDef<TFilters>
  | AsyncSelectFilterDef<TFilters>
  | BooleanFilterDef<TFilters>
  | DateRangeFilterDef<TFilters>

export interface FilterBarProps<TFilters extends Record<string, unknown>> {
  defs: FilterDef<TFilters>[]
  values: Partial<TFilters>
  onChange: (update: Partial<TFilters>) => void
}
