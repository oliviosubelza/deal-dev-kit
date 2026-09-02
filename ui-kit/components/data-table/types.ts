import type { ColumnPinningState, ColumnSizingState, SortingState, VisibilityState } from '@tanstack/react-table'
import type { ComponentType, ReactNode } from 'react'

// ── Densidad ──────────────────────────────────────────────────────────────────────────────────

export const DENSITY = { compact: 'compact', normal: 'normal', comfortable: 'comfortable' } as const
export type DensityMode = (typeof DENSITY)[keyof typeof DENSITY]

// ── Columnas ──────────────────────────────────────────────────────────────────────────────────

export interface ColumnMeta {
  align?: 'left' | 'center' | 'right'
  className?: string
}

export interface ColumnDefConfig<T> {
  id: string
  header: string
  accessorKey?: keyof T & string
  cell?: (row: T, index: number) => ReactNode
  enableSorting?: boolean
  enableResizing?: boolean
  enableHiding?: boolean
  size?: number
  minSize?: number
  maxSize?: number
  pin?: 'left' | 'right'
  meta?: ColumnMeta
}

export interface RowAction<T> {
  label: string
  icon?: ComponentType<{ size?: number; className?: string }>
  onClick: (row: T) => void
  variant?: 'default' | 'destructive'
  disabled?: (row: T) => boolean
  separator?: boolean
}

export interface BulkAction<T> {
  label: string
  icon?: ComponentType<{ size?: number; className?: string }>
  onClick: (rows: T[]) => void
  variant?: 'default' | 'destructive'
}

export interface ServerPagination {
  page: number
  limit: number
  total: number
  onPageChange: (page: number) => void
  onLimitChange?: (limit: number) => void
  pageSizeOptions?: number[]
}

/** Textos de la tabla (paginación, estados, toolbar). Sin esto, se usan los defaults en español. */
export interface DataTableLabels {
  errorTitle: string
  errorLoading: string
  emptyTitle: string
  empty: string
  search: string
  filters: string
  columns: string
  densityTitle: string
  densityLabel: string
  exportCsv: string
  reset: string
  retry: string
  range: (from: number, to: number, total: number) => string
  perPage: (size: number) => string
  rows: (count: number) => string
  first: string
  prev: string
  next: string
  last: string
}

export const DEFAULT_DATA_TABLE_LABELS: DataTableLabels = {
  errorTitle: 'No se pudo cargar la información',
  errorLoading: 'Ocurrió un error al cargar los datos.',
  emptyTitle: 'Sin resultados',
  empty: 'No hay datos para mostrar.',
  search: 'Buscar…',
  filters: 'Filtros',
  columns: 'Columnas',
  densityTitle: 'Densidad',
  densityLabel: 'Densidad',
  exportCsv: 'Exportar CSV',
  reset: 'Restablecer vista',
  retry: 'Reintentar',
  range: (from, to, total) => `${from}–${to} de ${total}`,
  perPage: (size) => `${size} por página`,
  rows: (count) => `${count} filas`,
  first: 'Primera página',
  prev: 'Página anterior',
  next: 'Página siguiente',
  last: 'Última página',
}

export interface DataTableProps<T extends object> {
  tableId: string
  columns: ColumnDefConfig<T>[]
  data: T[]
  getRowId?: (row: T) => string

  // Estado
  isLoading?: boolean
  isError?: boolean
  isRowLoading?: (row: T) => boolean
  isCellLoading?: (row: T, columnId: string) => boolean
  errorTitle?: string
  errorMessage?: string
  onRetry?: () => void
  emptyTitle?: string
  emptyMessage?: string
  emptyAction?: { label: string; onClick: () => void }
  emptySlot?: ReactNode
  bodyMinHeight?: number
  fillHeight?: boolean

  // Textos (sin esto, defaults en español — ver DEFAULT_DATA_TABLE_LABELS)
  labels?: Partial<DataTableLabels>

  // Orden inicial
  initialSort?: { id: string; desc?: boolean }

  // Paginación
  pagination?: ServerPagination
  clientPagination?: boolean
  defaultPageSize?: number

  // Selección
  selectable?: boolean
  onSelectionChange?: (rows: T[]) => void
  isRowSelectable?: (row: T) => boolean
  defaultSelectedIds?: string[]

  // Reordenamiento de filas (drag-and-drop, opt-in)
  enableRowReorder?: boolean
  onRowReorder?: (activeId: string, overId: string) => void

  // Acciones
  rowActions?: (row: T) => RowAction<T>[]
  bulkActions?: BulkAction<T>[]

  // Interacción de fila
  onRowClick?: (row: T) => void
  onRowDoubleClick?: (row: T) => void
  rowClassName?: (row: T) => string

  // Expandible
  expandable?: boolean
  renderExpanded?: (row: T) => ReactNode

  // Búsqueda
  searchable?: boolean
  searchPlaceholder?: string
  defaultSearch?: string
  searchKeys?: (keyof T & string)[]
  onSearchChange?: (value: string) => void

  // Exportar
  exportable?: boolean
  exportFilename?: string

  // Barra de filtros / toolbar extra
  filterBar?: ReactNode
  toolbar?: ReactNode

  // Apariencia
  defaultDensity?: DensityMode
  stickyHeader?: boolean
  striped?: boolean
}

// ── Estado persistido por tabla (uso interno de useTableState) ──────────────────────────────────

export interface PersistedTableState {
  columnOrder: string[]
  columnSizing: ColumnSizingState
  columnVisibility: VisibilityState
  columnPinning: ColumnPinningState
  sorting: SortingState
  density: DensityMode
  pageSize: number
}
