import { useState, useEffect, useCallback, useMemo, useRef, Fragment, createContext, useContext } from 'react'
import {
  DndContext,
  type DragOverEvent,
  type DragEndEvent,
  type Modifier,
  PointerSensor,
  useSensor,
  useSensors,
  closestCenter,
} from '@dnd-kit/core'
import { restrictToFirstScrollableAncestor } from '@dnd-kit/modifiers'
import {
  SortableContext,
  arrayMove,
  horizontalListSortingStrategy,
  verticalListSortingStrategy,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getExpandedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
  type VisibilityState,
  type RowSelectionState,
  type ColumnSizingState,
  type ColumnPinningState,
  type ExpandedState,
  type Header,
  type Cell,
  type Row,
} from '@tanstack/react-table'
import { cn } from '@/lib/utils'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ChevronDown,
  Inbox,
  Plus,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Columns3,
  Download,
  EyeOff,
  GripVertical,
  MoreHorizontal,
  Pin,
  PinOff,
  RotateCcw,
  Search,
  SlidersHorizontal,
  X,
  AlignJustify,
  Rows3,
  StretchHorizontal,
  ChevronRight as ExpandChevron,
} from 'lucide-react'
import { useTableState } from './useTableState'
import type { DataTableProps, ColumnDefConfig, RowAction, DensityMode, DataTableLabels } from './types'
import { DENSITY, DEFAULT_DATA_TABLE_LABELS } from './types'

const SPECIAL = new Set(['__select__', '__actions__', '__expand__', '__dragrow__'])

const DENSITY_ROW: Record<DensityMode, string> = {
  compact:     'h-7  text-[11px]',
  normal:      'h-9  text-xs',
  comfortable: 'h-12 text-sm',
}

const DENSITY_CELL: Record<DensityMode, string> = {
  compact:     'px-2 py-0',
  normal:      'px-2.5 py-1',
  comfortable: 'px-3 py-2',
}

const DENSITY_ICON: Record<DensityMode, typeof Rows3> = {
  compact:     Rows3,
  normal:      AlignJustify,
  comfortable: StretchHorizontal,
}

const DENSITY_LABEL: Record<DensityMode, string> = {
  compact:     'Compacta',
  normal:      'Normal',
  comfortable: 'Cómoda',
}

const PAGE_SIZES = [10, 20, 50, 100]

// Alto mínimo del cuadro de la tabla (header + cuerpo). Sin esto, una tabla vacía o en error
// colapsa a la altura de su cartel y todo lo que está debajo (mapa, acciones) salta hacia arriba
// cada vez que un fetch vuelve sin filas. Con esto el cuadro mide siempre lo mismo.
const DEFAULT_BODY_MIN_HEIGHT = 320

// Alto real del thead según densidad (h-7 / h-9 / h-12 en DraggableHeader) — se descuenta para que
// el cartel de vacío/error ocupe EXACTAMENTE el resto y el total dé bodyMinHeight clavado.
// MANTENER EN SINCRONÍA con DENSITY_ROW y con las clases del <th> en DraggableHeader.
const HEADER_HEIGHT: Record<DensityMode, number> = { compact: 28, normal: 36, comfortable: 48 }

function toTanstackCols<T extends object>(defs: ColumnDefConfig<T>[]): ColumnDef<T>[] {
  return defs.map((def) => ({
    id: def.id,
    header: def.header,
    ...(def.accessorKey ? { accessorKey: def.accessorKey } : { accessorFn: () => null }),
    cell: def.cell
      ? ({ row }: { row: Row<T> }) => (def.cell as NonNullable<typeof def.cell>)(row.original, row.index)
      : def.accessorKey
        ? ({ row }: { row: Row<T> }) => String((row.original as Record<string, unknown>)[def.accessorKey!] ?? '')
        : () => null,
    enableSorting:  def.enableSorting  ?? true,
    enableResizing: def.enableResizing ?? true,
    enableHiding:   def.enableHiding   ?? true,
    size:    def.size    ?? 150,
    minSize: def.minSize ?? 32,
    maxSize: def.maxSize ?? 600,
    meta:    def.meta,
  }))
}

function exportCSV<T extends object>(defs: ColumnDefConfig<T>[], data: T[], filename: string) {
  const cols = defs.filter((d) => d.accessorKey)
  const headers = cols.map((d) => d.header)
  const rows = data.map((row) =>
    cols.map((d) => {
      const val = d.accessorKey ? (row as Record<string, unknown>)[d.accessorKey] : ''
      return `"${String(val ?? '').replace(/"/g, '""')}"`
    })
  )
  const csv = [headers.join(','), ...rows.map((r) => r.join(','))].join('\n')
  const blob = new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${filename || 'export'}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

function DraggableHeader<T extends object>({
  header,
  density,
}: {
  header: Header<T, unknown>
  density: DensityMode
}) {
  const col = header.column
  const isSpecial = SPECIAL.has(col.id)
  const isPinned = col.getIsPinned()
  const meta = col.columnDef.meta as { align?: string } | undefined
  const align = meta?.align ?? 'left'

  const { attributes, listeners, setNodeRef, transform, isDragging } = useSortable({
    id: col.id,
    disabled: isSpecial,
  })

  const [resizing, setResizing] = useState(false)

  // Resize handler PROPIO: el de TanStack (`header.getResizeHandler`) ata mousemove/mouseup al
  // `document` GLOBAL (ventana principal). En una ventana secundaria (WindowPortal) la tabla vive en
  // OTRO document, así que esos eventos no llegan durante el drag y el resize "salta de golpe" recién
  // cuando el puntero vuelve a la principal. Atamos los listeners al ownerDocument del <th> → live en
  // ambas ventanas. Usa setColumnSizing del table (updater funcional → sin estados viejos).
  const startResize = (e: React.MouseEvent | React.TouchEvent) => {
    const startX = ('touches' in e ? e.touches[0] : e).clientX
    const startSize = col.getSize()
    const table = header.getContext().table
    const doc = (e.currentTarget as HTMLElement).ownerDocument
    const minSize = col.columnDef.minSize ?? 20
    const maxSize = col.columnDef.maxSize ?? Number.MAX_SAFE_INTEGER
    setResizing(true)
    const onMove = (ev: MouseEvent | TouchEvent) => {
      const clientX = ('touches' in ev ? ev.touches[0] : ev).clientX
      const next = Math.min(maxSize, Math.max(minSize, startSize + (clientX - startX)))
      table.setColumnSizing((prev) => ({ ...prev, [col.id]: next }))
    }
    const onEnd = () => {
      setResizing(false)
      doc.removeEventListener('mousemove', onMove)
      doc.removeEventListener('mouseup', onEnd)
      doc.removeEventListener('touchmove', onMove)
      doc.removeEventListener('touchend', onEnd)
    }
    doc.addEventListener('mousemove', onMove)
    doc.addEventListener('mouseup', onEnd)
    doc.addEventListener('touchmove', onMove, { passive: false })
    doc.addEventListener('touchend', onEnd)
    e.preventDefault()
    e.stopPropagation()
  }

  const pinStyle: React.CSSProperties = isPinned
    ? {
        position: 'sticky',
        left:  isPinned === 'left'  ? `${col.getStart('left')}px`  : undefined,
        right: isPinned === 'right' ? `${col.getAfter('right')}px` : undefined,
        zIndex: 3,
      }
    : {}

  const style: React.CSSProperties = {
    width: header.getSize(),
    transform: CSS.Transform.toString(transform ? { ...transform, scaleY: 1 } : null),
    transition: isDragging ? undefined : 'transform 150ms ease',
    opacity: isDragging ? 0.85 : 1,
    ...pinStyle,
  }

  return (
    <th
      ref={setNodeRef}
      style={style}
      className={cn(
        'relative border-b border-r last:border-r-0 bg-muted/50 select-none group/th',
        density === 'compact' ? 'h-7 text-[11px]' : density === 'normal' ? 'h-9 text-xs' : 'h-12 text-xs',
        isDragging && 'bg-accent z-10',
        isPinned && 'bg-muted/70',
      )}
    >
      <div
        className={cn(
          'flex items-center gap-1 px-2 font-medium text-muted-foreground',
          align === 'center' && 'justify-center',
          align === 'right'  && 'justify-end',
        )}
      >
        {/* Drag handle */}
        {!isSpecial && (
          <button
            {...attributes}
            {...listeners}
            className="cursor-grab active:cursor-grabbing opacity-30 hover:opacity-70 shrink-0 -ml-1 p-0.5"
            tabIndex={-1}
          >
            <GripVertical size={12} />
          </button>
        )}

        {/* Header label + sort */}
        {!header.isPlaceholder && (
          <span
            className={cn(
              'flex-1 truncate leading-none',
              col.getCanSort() && 'cursor-pointer hover:text-foreground transition-colors',
            )}
            onClick={col.getCanSort() ? col.getToggleSortingHandler() : undefined}
          >
            {flexRender(col.columnDef.header, header.getContext())}
          </span>
        )}

        {col.getCanSort() && (
          <span
            className="shrink-0 cursor-pointer text-muted-foreground/60 hover:text-foreground transition-colors"
            onClick={col.getToggleSortingHandler()}
          >
            {col.getIsSorted() === 'asc'  ? <ArrowUp size={11} /> :
             col.getIsSorted() === 'desc' ? <ArrowDown size={11} /> :
             <ArrowUpDown size={11} className="opacity-40" />}
          </span>
        )}

        {/* Column options dropdown */}
        {!isSpecial && (
          <DropdownMenu>
            <DropdownMenuTrigger
              className="h-5 w-5 inline-flex items-center justify-center rounded opacity-0 group-hover/th:opacity-100 transition-opacity shrink-0 hover:bg-accent/80"
            >
              <ChevronDown size={10} />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-44">
              {col.getCanSort() && (
                <>
                  <DropdownMenuItem onClick={() => col.toggleSorting(false)}>
                    <ArrowUp size={13} className="mr-2 shrink-0" /> Ordenar A → Z
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => col.toggleSorting(true)}>
                    <ArrowDown size={13} className="mr-2 shrink-0" /> Ordenar Z → A
                  </DropdownMenuItem>
                  {col.getIsSorted() && (
                    <DropdownMenuItem onClick={() => col.clearSorting()}>
                      <ArrowUpDown size={13} className="mr-2 shrink-0" /> Quitar orden
                    </DropdownMenuItem>
                  )}
                  <DropdownMenuSeparator />
                </>
              )}
              {isPinned ? (
                <DropdownMenuItem onClick={() => col.pin(false)}>
                  <PinOff size={13} className="mr-2 shrink-0" /> Desanclar
                </DropdownMenuItem>
              ) : (
                <>
                  <DropdownMenuItem onClick={() => col.pin('left')}>
                    <Pin size={13} className="mr-2 shrink-0" /> Anclar izquierda
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => col.pin('right')}>
                    <Pin size={13} className="mr-2 shrink-0" /> Anclar derecha
                  </DropdownMenuItem>
                </>
              )}
              {col.getCanHide() && (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => col.toggleVisibility(false)}>
                    <EyeOff size={13} className="mr-2 shrink-0" /> Ocultar columna
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      {/* Resize handle */}
      {col.getCanResize() && (
        <div
          onMouseDown={startResize}
          onTouchStart={startResize}
          className={cn(
            'absolute right-0 top-0 h-full w-1 cursor-col-resize select-none touch-none',
            'bg-transparent hover:bg-primary/40 transition-colors',
            (col.getIsResizing() || resizing) && 'bg-primary',
          )}
        />
      )}
    </th>
  )
}

function RowActionsMenu<T extends object>({ row, actions }: { row: T; actions: RowAction<T>[] }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="h-7 w-7 inline-flex items-center justify-center rounded-md hover:bg-accent transition-colors"
        onClick={(e: React.MouseEvent) => e.stopPropagation()}
        onPointerDown={(e: React.PointerEvent) => e.stopPropagation()}
      >
        <MoreHorizontal size={14} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40">
        {actions.map((action, i) => (
          <Fragment key={i}>
            {action.separator && i > 0 && <DropdownMenuSeparator />}
            <DropdownMenuItem
              disabled={action.disabled?.(row)}
              onClick={(e) => {
                e.stopPropagation()
                action.onClick(row)
              }}
              className={action.variant === 'destructive' ? 'text-destructive focus:text-destructive' : ''}
            >
              {action.icon && <action.icon size={13} className="mr-2 shrink-0" />}
              {action.label}
            </DropdownMenuItem>
          </Fragment>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function pinnedStyle(col: {
  getIsPinned: () => false | 'left' | 'right'
  getStart:    (s: 'left')  => number
  getAfter:    (s: 'right') => number
}): React.CSSProperties {
  const pin = col.getIsPinned()
  if (!pin) return {}
  return {
    position: 'sticky',
    left:  pin === 'left'  ? `${col.getStart('left')}px`  : undefined,
    right: pin === 'right' ? `${col.getAfter('right')}px` : undefined,
    zIndex: 1,
  }
}

// Celda del cuerpo que se traslada junto a su cabecera durante el reordenamiento por DnD.
// Usa el mismo `useSortable` (por id de columna) que DraggableHeader; debe vivir dentro de un
// SortableContext por fila. Las columnas fijadas (pinned/especiales) mantienen su sticky.
function DragAlongCell<T extends object>({
  cell,
  density,
  loading,
}: {
  cell: Cell<T, unknown>
  density: DensityMode
  /** Pinta la celda como skeleton (fila o celda "cargando") en vez de su contenido real. */
  loading?: boolean
}) {
  const col = cell.column
  const isSpecial = SPECIAL.has(col.id)
  const pin = col.getIsPinned()
  const cMeta = col.columnDef.meta as { align?: string; className?: string } | undefined

  const { setNodeRef, transform, isDragging } = useSortable({ id: col.id, disabled: isSpecial })

  const style: React.CSSProperties = pin
    ? { width: col.getSize(), ...pinnedStyle(col) }
    : {
        width: col.getSize(),
        transform: CSS.Transform.toString(transform ? { ...transform, scaleY: 1 } : null),
        transition: isDragging ? undefined : 'transform 150ms ease',
        ...(isDragging ? { position: 'relative', zIndex: 1 } : {}),
      }

  return (
    <td
      ref={setNodeRef}
      style={style}
      onClick={(e) => {
        if (
          col.id === '__dragrow__' ||
          col.id === '__actions__' ||
          col.id === 'seq' ||
          col.id === '__expand__' ||
          col.id === '__select__'
        ) {
          e.stopPropagation()
        }
      }}
      className={cn(
        DENSITY_CELL[density],
        'border-r last:border-r-0',
        pin && 'bg-[inherit]',
        // Fondo opaco mientras se arrastra para que no se transparente sobre las vecinas.
        isDragging && 'bg-background',
        cMeta?.align === 'center' && 'text-center',
        // Derecha = convención numérica (precios, cantidades, totales, kardex) → cifras tabulares.
        cMeta?.align === 'right'  && 'text-right tabular-nums',
        cMeta?.className,
      )}
    >
      {loading ? <Skeleton className="h-3 w-full" /> : flexRender(cell.column.columnDef.cell, cell.getContext())}
    </td>
  )
}

// ── Reordenamiento de FILAS (opt-in) ──
// El drag de filas convive con el de columnas en UN solo DndContext (evita anidar contextos, que
// mete <div> ocultos inválidos dentro de <tbody>). El eje se decide con un modifier según lo que se
// arrastra; los handlers distinguen fila vs. columna por el id. El "grip" de cada fila vive en una
// celda especial (__dragrow__) y toma los listeners de su fila vía este contexto.
type RowDragBindings = Pick<ReturnType<typeof useSortable>, 'attributes' | 'listeners'>
const RowDragContext = createContext<RowDragBindings | null>(null)

function RowDragHandle() {
  const ctx = useContext(RowDragContext)
  if (!ctx) return null
  return (
    <button
      {...ctx.attributes}
      {...ctx.listeners}
      onClick={(e) => e.stopPropagation()}
      className="cursor-grab active:cursor-grabbing opacity-40 hover:opacity-80 p-0.5"
      tabIndex={-1}
      aria-label="Reordenar fila"
    >
      <GripVertical size={13} />
    </button>
  )
}

/** <tr> arrastrable verticalmente. Provee sus listeners al grip de la celda __dragrow__. */
function SortableDataRow<T>({
  rowId,
  className,
  onClick,
  onDoubleClick,
  children,
}: {
  rowId: string
  className?: string
  onClick?: () => void
  onDoubleClick?: () => void
  children: React.ReactNode
}) {
  const { setNodeRef, transform, transition, isDragging, attributes, listeners } = useSortable({ id: rowId })
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform ? { ...transform, scaleX: 1 } : null),
    transition,
    ...(isDragging ? { position: 'relative', zIndex: 1 } : {}),
  }
  return (
    <RowDragContext.Provider value={{ attributes, listeners }}>
      <tr ref={setNodeRef} style={style} className={className} onClick={onClick} onDoubleClick={onDoubleClick}>
        {children}
      </tr>
    </RowDragContext.Provider>
  )
}

export function DataTable<T extends object>({
  tableId,
  columns: columnDefs,
  data,
  getRowId,

  isLoading,
  isRowLoading,
  isCellLoading,
  isError,
  errorTitle: errorTitleProp,
  errorMessage: errorMessageProp,
  onRetry,
  emptyTitle: emptyTitleProp,
  emptyMessage: emptyMessageProp,
  emptyAction,
  emptySlot,
  bodyMinHeight = DEFAULT_BODY_MIN_HEIGHT,
  fillHeight,

  pagination: serverPagination,
  clientPagination,
  defaultPageSize,
  initialSort,

  selectable,
  onSelectionChange,
  isRowSelectable,
  defaultSelectedIds,

  enableRowReorder,
  onRowReorder,

  rowActions,
  bulkActions,

  onRowClick,
  onRowDoubleClick,
  rowClassName,

  expandable,
  renderExpanded,

  searchable,
  searchPlaceholder: searchPlaceholderProp,
  defaultSearch = '',
  onSearchChange,
  searchKeys,

  exportable,
  exportFilename,

  filterBar,
  toolbar,

  defaultDensity,
  stickyHeader,
  striped,

  labels: labelsProp,
}: DataTableProps<T>) {
  const t: DataTableLabels = { ...DEFAULT_DATA_TABLE_LABELS, ...labelsProp }
  const errorTitle = errorTitleProp ?? t.errorTitle
  const errorMessage = errorMessageProp ?? t.errorLoading
  const emptyTitle = emptyTitleProp ?? t.emptyTitle
  const emptyMessage = emptyMessageProp ?? t.empty
  const searchPlaceholder = searchPlaceholderProp ?? t.search
  const { state: ts, persist, reset, isLoaded } = useTableState(tableId, defaultDensity, defaultPageSize)

  // Selección inicial desde `defaultSelectedIds` (initializer perezoso: se evalúa solo al montar,
  // así una prop nueva NO re-siembra — para eso, remontar la tabla con otra `key`).
  const [rowSelection,      setRowSelection]      = useState<RowSelectionState>(() =>
    Object.fromEntries((defaultSelectedIds ?? []).map((id) => [id, true]))
  )
  const [globalFilter,      setGlobalFilter]      = useState(defaultSearch)
  const [expanded,          setExpanded]          = useState<ExpandedState>({})
  // Orden por defecto = initialSort (hasta que el usuario ordene otra columna, que se persiste).
  const [sorting,           setSorting]           = useState<SortingState>(
    initialSort ? [{ id: initialSort.id, desc: initialSort.desc ?? false }] : [],
  )
  const [columnVisibility,  setColumnVisibility]  = useState<VisibilityState>({})
  const [columnSizing,      setColumnSizing]      = useState<ColumnSizingState>({})
  const [columnOrder,       setColumnOrder]       = useState<string[]>([])
  const [columnPinning,     setColumnPinning]     = useState<ColumnPinningState>(() => ({
    left:  columnDefs.filter((d) => d.pin === 'left').map((d) => d.id),
    right: columnDefs.filter((d) => d.pin === 'right').map((d) => d.id),
  }))
  const [density,           setDensity]           = useState<DensityMode>(defaultDensity ?? 'compact')
  const [pageSize,          setPageSize]          = useState(defaultPageSize ?? 20)
  const [clientPage,        setClientPage]        = useState(0)

  // Load persisted state after storage resolves
  useEffect(() => {
    if (!isLoaded) return
    // El orden persistido del usuario gana; si nunca ordenó (vacío), se respeta el initialSort.
    if (ts.sorting.length > 0) setSorting(ts.sorting)
    setColumnVisibility(ts.columnVisibility)
    setColumnSizing(ts.columnSizing)
    setColumnOrder(ts.columnOrder)
    setColumnPinning(ts.columnPinning)
    setDensity(ts.density)
    setPageSize(ts.pageSize)
  }, [isLoaded])

  // Persist each state slice on change
  useEffect(() => { if (isLoaded) persist({ sorting }) },          [sorting])
  useEffect(() => { if (isLoaded) persist({ columnVisibility }) }, [columnVisibility])
  useEffect(() => { if (isLoaded) persist({ columnSizing }) },     [columnSizing])
  useEffect(() => { if (isLoaded) persist({ columnOrder }) },      [columnOrder])
  useEffect(() => { if (isLoaded) persist({ columnPinning }) },    [columnPinning])
  useEffect(() => { if (isLoaded) persist({ density }) },          [density])
  useEffect(() => { if (isLoaded) persist({ pageSize }) },         [pageSize])

  // Build column definitions
  const builtCols = toTanstackCols(columnDefs)

  const selectCol: ColumnDef<T> = {
    id: '__select__',
    header: ({ table }) => (
      <Checkbox
        checked={table.getIsAllPageRowsSelected()}
        onCheckedChange={(v) => table.toggleAllPageRowsSelected(!!v)}
        aria-label="Seleccionar todo"
      />
    ),
    cell: ({ row }) => (
      <Checkbox
        checked={row.getIsSelected()}
        disabled={!row.getCanSelect()}
        onCheckedChange={(v) => row.toggleSelected(!!v)}
        aria-label="Seleccionar fila"
        onClick={(e) => e.stopPropagation()}
      />
    ),
    enableSorting: false, enableResizing: false, enableHiding: false,
    size: 40, minSize: 40, maxSize: 40,
  }

  const actionsCol: ColumnDef<T> = {
    id: '__actions__',
    header: '',
    cell: ({ row }) => rowActions
      ? <RowActionsMenu row={row.original} actions={rowActions(row.original)} />
      : null,
    enableSorting: false, enableResizing: false, enableHiding: false,
    size: 48, minSize: 48, maxSize: 48,
  }

  const expandCol: ColumnDef<T> = {
    id: '__expand__',
    header: '',
    cell: ({ row }) => row.getCanExpand() ? (
      <button
        className="h-6 w-6 inline-flex items-center justify-center rounded hover:bg-accent transition-colors"
        onClick={(e) => { e.stopPropagation(); row.toggleExpanded() }}
      >
        <ExpandChevron size={13} className={cn('transition-transform', row.getIsExpanded() && 'rotate-90')} />
      </button>
    ) : null,
    enableSorting: false, enableResizing: false, enableHiding: false,
    size: 40, minSize: 40, maxSize: 40,
  }

  const dragCol: ColumnDef<T> = {
    id: '__dragrow__',
    header: '',
    cell: () => <RowDragHandle />,
    enableSorting: false, enableResizing: false, enableHiding: false,
    size: 36, minSize: 36, maxSize: 36,
  }

  const allCols: ColumnDef<T>[] = [
    ...(enableRowReorder ? [dragCol] : []),
    ...(selectable ? [selectCol]  : []),
    ...(expandable ? [expandCol]  : []),
    ...builtCols,
    ...(rowActions ? [actionsCol] : []),
  ]

  const useServerPagination = !!serverPagination
  const useClientPagination = !useServerPagination && !!clientPagination
  const effectiveOrder = columnOrder.length ? columnOrder : allCols.map((c) => c.id!)

  const table = useReactTable<T>({
    data,
    columns: allCols,
    state: {
      sorting,
      columnVisibility,
      columnSizing,
      columnOrder: effectiveOrder,
      columnPinning,
      rowSelection,
      globalFilter,
      expanded,
      ...(useClientPagination ? { pagination: { pageIndex: clientPage, pageSize } } : {}),
    },
    columnResizeMode: 'onChange',
    // TanStack resetea la página (y el expandido) cada vez que cambia la IDENTIDAD de `data`:
    // `getRowModel`/`getFilteredRowModel`/`getSortedRowModel` están memoizados por referencia y su
    // callback llama a `_autoResetPageIndex()`. Con el default (`!manualPagination` → true) alcanzaba
    // con que el caller reconstruyera el array en el render para que la tabla saltara a la página 1:
    // tildar una fila en la página 3 dispara onSelectionChange → el padre re-renderiza → `data` es un
    // array nuevo → de vuelta a la página 1 con la selección invisible.
    //
    // La página es estado de NAVEGACIÓN del usuario: no puede depender de si el caller memoiza su
    // array. Se apaga el reset automático y abajo se resetea explícitamente donde sí corresponde
    // (búsqueda) y se acota la página cuando deja de existir.
    autoResetPageIndex: false,
    autoResetExpanded: false,
    // `isRowSelectable` restringe la selección fila por fila (tanstack acepta la forma función);
    // sin ella, el comportamiento es el de siempre (todas seleccionables si `selectable`).
    enableRowSelection:      isRowSelectable ? (row) => isRowSelectable(row.original) : !!selectable,
    enableMultiRowSelection: !!selectable,
    getRowId,
    getRowCanExpand: expandable ? () => true : undefined,
    onSortingChange:          (u) => setSorting(u instanceof Function ? u(sorting) : u),
    onColumnVisibilityChange: (u) => setColumnVisibility(u instanceof Function ? u(columnVisibility) : u),
    // Setter directo: TanStack pasa value-o-updater, igual que acepta el setState → updates
    // funcionales correctos (clave durante el resize live, sin leer un columnSizing viejo del closure).
    onColumnSizingChange:     setColumnSizing,
    onColumnOrderChange:      (u) => setColumnOrder(u instanceof Function ? u(effectiveOrder) : u),
    onColumnPinningChange:    (u) => setColumnPinning(u instanceof Function ? u(columnPinning) : u),
    onRowSelectionChange:     (u) => setRowSelection(u instanceof Function ? u(rowSelection) : u),
    onGlobalFilterChange:     setGlobalFilter,
    // Con `searchKeys` la búsqueda global mira los CAMPOS de la fila y no las columnas. TanStack
    // invoca esta fn una vez por columna filtrable y hace OR de los resultados, así que se ignora
    // `columnId` y se responde por fila: el primer true corta el recorrido.
    ...(searchKeys?.length
      ? {
          globalFilterFn: (row: Row<T>, _columnId: string, filterValue: unknown) => {
            const needle = String(filterValue ?? '').trim().toLowerCase()
            if (!needle) return true
            const data = row.original as Record<string, unknown>
            return searchKeys.some((key) =>
              String(data[key] ?? '').toLowerCase().includes(needle)
            )
          },
        }
      : {}),
    onExpandedChange:         (u) => setExpanded(u instanceof Function ? u(expanded) : u),
    ...(useClientPagination ? {
      onPaginationChange: (u) => {
        const next = u instanceof Function ? u({ pageIndex: clientPage, pageSize }) : u
        setClientPage(next.pageIndex)
        setPageSize(next.pageSize)
      },
      getPaginationRowModel: getPaginationRowModel(),
    } : {}),
    getCoreRowModel:     getCoreRowModel(),
    getSortedRowModel:   getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getExpandedRowModel: expandable ? getExpandedRowModel() : undefined,
    manualPagination:    useServerPagination,
    ...(useServerPagination ? { rowCount: serverPagination.total } : {}),
  })

  // Buscar cambia el conjunto de resultados: volver a la primera página es lo que el usuario espera
  // (quedarse en la página 7 de un resultado de 2 páginas se ve como "no encontró nada").
  const primeraBusqueda = useRef(true)
  useEffect(() => {
    if (primeraBusqueda.current) {
      primeraBusqueda.current = false
      return
    }
    setClientPage(0)
  }, [globalFilter])

  // La página actual puede dejar de existir porque el caller filtró, borró filas o achicó el dataset.
  // Se acota a la última página válida en vez de saltar a la primera: el usuario pierde lo mínimo.
  const clientPageCount = useClientPagination ? table.getPageCount() : 0
  useEffect(() => {
    if (!useClientPagination) return
    if (clientPageCount === 0) {
      if (clientPage !== 0) setClientPage(0)
      return
    }
    if (clientPage > clientPageCount - 1) setClientPage(clientPageCount - 1)
  }, [clientPage, clientPageCount, useClientPagination])

  // Selección REAL sobre `data`: no depende de la página visible ni del row model filtrado.
  // Para tablas con paginación/filtros (como "fuera de corte"), usar `getSelectedRowModel()` podía
  // perder filas ya seleccionadas pero no visibles y el caller las interpretaba como destildadas.
  /**
   * Id de fila con el MISMO fallback que TanStack: si el consumidor no pasó `getRowId` —es opcional en
   * el tipo—, la tabla usa el índice, así que acá hay que usar el índice o los dos lados hablan de
   * filas distintas. Sin esto, además, `getRowId(row)` se llamaba sin guarda y cualquier tabla con
   * `selectable` y sin `getRowId` reventaba con "getRowId is not a function".
   */
  const idDeFila = useCallback(
    (row: T, index: number) => getRowId?.(row) ?? String(index),
    [getRowId],
  )
  const selectedRows = useMemo(() => {
    if (!selectable) return []
    return data.filter((row, i) => !!rowSelection[idDeFila(row, i)])
  }, [data, idDeFila, rowSelection, selectable])
  const selectionSignature = useMemo(
    () => selectedRows.map((row, i) => idDeFila(row, i)).join('|'),
    [idDeFila, selectedRows],
  )
  const onSelectionChangeRef = useRef(onSelectionChange)
  const selectedRowsRef = useRef(selectedRows)

  useEffect(() => {
    onSelectionChangeRef.current = onSelectionChange
  }, [onSelectionChange])

  useEffect(() => {
    selectedRowsRef.current = selectedRows
  }, [selectedRows])

  // Fire selection callback
  useEffect(() => {
    if (!onSelectionChangeRef.current) return
    onSelectionChangeRef.current(selectedRowsRef.current)
  }, [selectionSignature])

  // Keep special columns pinned
  useEffect(() => {
    const left  = [
      ...(enableRowReorder ? ['__dragrow__'] : []),
      ...(selectable ? ['__select__'] : []),
      ...(expandable ? ['__expand__'] : []),
      ...(columnPinning.left?.filter((id) => !SPECIAL.has(id)) ?? []),
    ]
    const right = [
      ...(columnPinning.right?.filter((id) => !SPECIAL.has(id)) ?? []),
      ...(rowActions ? ['__actions__'] : []),
    ]
    table.setColumnPinning({ left, right })
  }, [selectable, expandable, !!rowActions, enableRowReorder])

  // DnD setup
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))
  const draggableIds = effectiveOrder.filter((id) => !SPECIAL.has(id))

  function handleDragOver(event: DragOverEvent) {
    const { active, over } = event
    if (!active || !over || active.id === over.id) return
    // Filas: se reordenan al soltar (handleDragEnd), no en vivo. Columnas: en vivo, acá.
    if (rowIdSet.has(String(active.id))) return
    const order = table.getState().columnOrder
    const oldIdx = order.indexOf(active.id as string)
    const newIdx = order.indexOf(over.id as string)
    if (oldIdx !== -1 && newIdx !== -1) setColumnOrder(arrayMove(order, oldIdx, newIdx))
  }

  function handleDragEnd(event: DragEndEvent) {
    // Columnas ya se movieron en vivo. Solo avisamos el reordenamiento de FILAS al consumidor.
    const { active, over } = event
    if (!over || active.id === over.id) return
    if (rowIdSet.has(String(active.id)) && rowIdSet.has(String(over.id))) {
      onRowReorder?.(String(active.id), String(over.id))
    }
  }

  const hasSelection  = selectedRows.length > 0
  const DensityIcon   = DENSITY_ICON[density]
  // Lo que le queda al cartel de vacío/error una vez descontado el thead: header + celda = bodyMinHeight.
  const stateCellHeight = Math.max(0, bodyMinHeight - HEADER_HEIGHT[density])

  const visibleUserCols = table.getAllLeafColumns().filter((c) => !SPECIAL.has(c.id) && c.getCanHide())

  const { rows } = useServerPagination
    ? table.getRowModel()
    : useClientPagination
      ? table.getPaginationRowModel()
      : table.getRowModel()

  // Reordenamiento de filas: ids de las filas VISIBLES (solo esas se arrastran) + modifier que fija
  // el eje del drag (filas = vertical, columnas = horizontal) sobre un único DndContext.
  const rowIdSet = new Set(rows.map((r) => r.id))
  const axisLockModifier: Modifier = ({ transform, active }) =>
    active && rowIdSet.has(String(active.id))
      ? { ...transform, x: 0 }
      : { ...transform, y: 0 }

  // Una fila de datos (con o sin drag). Comparte className, celdas y fila expandida entre ambos modos.
  const renderDataRow = (row: Row<T>, rowIdx: number) => {
    const className = cn(
      DENSITY_ROW[density],
      'border-b transition-colors',
      (onRowClick || onRowDoubleClick) && 'cursor-pointer hover:bg-accent/50',
      !onRowClick && !onRowDoubleClick && 'hover:bg-muted/20',
      row.getIsSelected() && 'bg-primary/5',
      striped && rowIdx % 2 === 1 && 'bg-muted/20',
      rowClassName?.(row.original),
    )
    // Skeleton por fila o por celda: la fila entera carga (isRowLoading) o solo columnas puntuales
    // (isCellLoading). Basta que una de las dos dé true para esa celda.
    const rowLoading = isRowLoading?.(row.original) ?? false
    const cells = (
      <SortableContext items={draggableIds} strategy={horizontalListSortingStrategy}>
        {row.getVisibleCells().map((cell) => (
          <DragAlongCell
            key={cell.id}
            cell={cell}
            density={density}
            loading={rowLoading || (isCellLoading?.(row.original, cell.column.id) ?? false)}
          />
        ))}
      </SortableContext>
    )
    return (
      <Fragment key={row.id}>
        {enableRowReorder ? (
          <SortableDataRow
            rowId={row.id}
            className={className}
            onClick={onRowClick ? () => onRowClick(row.original) : undefined}
            onDoubleClick={onRowDoubleClick ? () => onRowDoubleClick(row.original) : undefined}
          >
            {cells}
          </SortableDataRow>
        ) : (
          <tr
            className={className}
            onClick={() => onRowClick?.(row.original)}
            onDoubleClick={() => onRowDoubleClick?.(row.original)}
          >
            {cells}
          </tr>
        )}

        {/* Expanded content */}
        {expandable && row.getIsExpanded() && renderExpanded && (
          <tr className="border-b bg-muted/20">
            <td colSpan={table.getVisibleLeafColumns().length} className="px-4 py-3">
              {renderExpanded(row.original)}
            </td>
          </tr>
        )}
      </Fragment>
    )
  }

  function handleReset() {
    reset()
    setSorting([])
    setColumnVisibility({})
    setColumnSizing({})
    setColumnOrder([])
    setColumnPinning({
      left:  columnDefs.filter((d) => d.pin === 'left').map((d) => d.id),
      right: columnDefs.filter((d) => d.pin === 'right').map((d) => d.id),
    })
    setDensity(defaultDensity ?? 'compact')
    setPageSize(defaultPageSize ?? 20)
    setGlobalFilter('')
    setRowSelection({})
    setClientPage(0)
  }

  return (
    <div className={cn('flex flex-col gap-2', fillHeight && 'min-h-0 flex-1')}>

      {/* ── Filter bar ── */}
      {filterBar && (
        <div className={cn('flex items-center gap-2 flex-wrap rounded-md border border-border/60 bg-muted/20 px-3 py-1.5', fillHeight && 'shrink-0')}>
          <span className="shrink-0 flex items-center gap-1.5 text-xs font-medium text-muted-foreground/80 pr-0.5">
            <SlidersHorizontal size={11} />
            {t.filters}
          </span>
          <Separator orientation="vertical" className="h-4 shrink-0" />
          {filterBar}
        </div>
      )}

      {/* ── Toolbar ── */}
      <div className={cn('flex items-center gap-2 flex-wrap', fillHeight && 'shrink-0')}>

        {/* Left: search + selection info + bulk actions */}
        <div className="flex items-center gap-2 flex-1 min-w-0">
          {searchable && (
            <div className="relative flex-1 max-w-xs">
              <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
              <Input
                placeholder={searchPlaceholder}
                value={globalFilter}
                onChange={(e) => {
                  setGlobalFilter(e.target.value)
                  onSearchChange?.(e.target.value)
                }}
                className="pl-8 h-8 text-sm"
              />
              {globalFilter && (
                <button
                  onClick={() => { setGlobalFilter(''); onSearchChange?.('') }}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  <X size={12} />
                </button>
              )}
            </div>
          )}

          {selectable && hasSelection && (
            <>
              <Badge variant="secondary" className="shrink-0">
                {selectedRows.length} seleccionado{selectedRows.length !== 1 ? 's' : ''}
              </Badge>
              <Button
                variant="ghost" size="sm"
                className="h-8 px-2 text-xs text-muted-foreground"
                onClick={() => table.resetRowSelection()}
              >
                <X size={11} className="mr-1" /> Limpiar
              </Button>
              {bulkActions?.map((action, i) => (
                <Button
                  key={i}
                  size="sm"
                  variant={action.variant === 'destructive' ? 'destructive' : 'outline'}
                  className="h-8 text-xs"
                  onClick={() => action.onClick(selectedRows)}
                >
                  {action.icon && <action.icon size={13} className="mr-1.5" />}
                  {action.label}
                </Button>
              ))}
            </>
          )}
        </div>

        {/* Right: toolbar slot + controls */}
        <div className="flex items-center gap-1 shrink-0">
          {toolbar}
          {toolbar && <Separator orientation="vertical" className="h-5 mx-1" />}

          {/* Column visibility */}
          <DropdownMenu>
            <DropdownMenuTrigger
              title={t.columns}
              className={cn(buttonVariants({ variant: 'outline', size: 'icon' }))}
            >
              <Columns3 size={14} />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48 max-h-72 overflow-y-auto">
              <DropdownMenuGroup>
                <DropdownMenuLabel className="text-xs">{t.columns}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                {visibleUserCols.map((col) => (
                  <DropdownMenuCheckboxItem
                    key={col.id}
                    checked={col.getIsVisible()}
                    onCheckedChange={(v) => col.toggleVisibility(v)}
                    className="text-sm"
                  >
                    {String(col.columnDef.header ?? col.id)}
                  </DropdownMenuCheckboxItem>
                ))}
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Density */}
          <DropdownMenu>
            <DropdownMenuTrigger
              title={t.densityTitle}
              className={cn(buttonVariants({ variant: 'outline', size: 'icon' }))}
            >
              <DensityIcon size={14} />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-36">
              <DropdownMenuGroup>
                <DropdownMenuLabel className="text-xs">{t.densityLabel}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuRadioGroup value={density} onValueChange={(v) => setDensity(v as DensityMode)}>
                  {(Object.keys(DENSITY) as DensityMode[]).map((d) => {
                    const Icon = DENSITY_ICON[d]
                    return (
                      <DropdownMenuRadioItem key={d} value={d} className="text-sm">
                        <Icon size={13} className="mr-2 shrink-0" />
                        {DENSITY_LABEL[d]}
                      </DropdownMenuRadioItem>
                    )
                  })}
                </DropdownMenuRadioGroup>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Export */}
          {exportable && (
            <Tooltip>
              <TooltipTrigger
                className={cn(buttonVariants({ variant: 'outline', size: 'icon' }))}
                onClick={() => exportCSV(columnDefs, data, exportFilename || tableId)}
              >
                <Download size={14} />
              </TooltipTrigger>
              <TooltipContent>{t.exportCsv}</TooltipContent>
            </Tooltip>
          )}

          {/* Reset */}
          <Tooltip>
            <TooltipTrigger
              className={cn(buttonVariants({ variant: 'outline', size: 'icon' }))}
              onClick={handleReset}
            >
              <RotateCcw size={14} />
            </TooltipTrigger>
            <TooltipContent>{t.reset}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {/* ── Table ── */}
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        modifiers={[axisLockModifier, restrictToFirstScrollableAncestor]}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
      <div className={cn('rounded-md border overflow-hidden', fillHeight && 'flex min-h-0 flex-1 flex-col')}>
        <div
          className={cn('overflow-auto', fillHeight && 'min-h-0 flex-1')}
          style={fillHeight ? undefined : { minHeight: bodyMinHeight }}
        >
          <table
            className={cn(
              'border-collapse',
              // En modo fillHeight, cuando NO hay filas (vacío/error) la tabla ocupa toda la altura
              // del cuadro flex para que la celda de estado se centre y la paginación quede abajo.
              fillHeight && !isLoading && (isError || rows.length === 0) && 'h-full',
            )}
            style={{ width: `max(${table.getTotalSize()}px, 100%)` }}
          >
            <thead className={cn(stickyHeader && 'sticky top-0 z-10')}>
              {table.getHeaderGroups().map((hg) => (
                <SortableContext key={hg.id} items={draggableIds} strategy={horizontalListSortingStrategy}>
                  <tr>
                    {hg.headers.map((header) => (
                      <DraggableHeader key={header.id} header={header} density={density} />
                    ))}
                  </tr>
                </SortableContext>
              ))}
            </thead>

            <tbody>
              {/* Loading skeleton */}
              {isLoading && Array.from({ length: 6 }).map((_, i) => (
                <tr key={i} className={cn(DENSITY_ROW[density], striped && i % 2 === 1 && 'bg-muted/20')}>
                  {table.getVisibleLeafColumns().map((col) => (
                    <td key={col.id} className={cn(DENSITY_CELL[density], 'border-b border-r last:border-r-0')}>
                      <Skeleton className="h-3 w-full" />
                    </td>
                  ))}
                </tr>
              ))}

              {/* Error state */}
              {!isLoading && isError && (
                <tr>
                  <td
                    colSpan={table.getVisibleLeafColumns().length}
                    style={{ height: fillHeight ? undefined : stateCellHeight }}
                    className="align-middle"
                  >
                    <Empty className="border-0">
                      <EmptyHeader>
                        <EmptyMedia
                          variant="icon"
                          className="size-10 bg-destructive/10 text-destructive [&_svg:not([class*='size-'])]:size-5"
                        >
                          <AlertTriangle />
                        </EmptyMedia>
                        <EmptyTitle>{errorTitle}</EmptyTitle>
                        <EmptyDescription>{errorMessage}</EmptyDescription>
                      </EmptyHeader>
                      {onRetry && (
                        <EmptyContent>
                          <Button variant="outline" size="sm" onClick={onRetry}>
                            <RotateCcw size={13} className="mr-1.5" />
                            {t.retry}
                          </Button>
                        </EmptyContent>
                      )}
                    </Empty>
                  </td>
                </tr>
              )}

              {/* Empty state */}
              {!isLoading && !isError && rows.length === 0 && (
                <tr>
                  <td
                    colSpan={table.getVisibleLeafColumns().length}
                    style={{ height: fillHeight ? undefined : stateCellHeight }}
                    className="align-middle"
                  >
                    {emptySlot ?? (
                      <Empty className="border-0">
                        <EmptyHeader>
                          <EmptyMedia
                            variant="icon"
                            className="size-10 [&_svg:not([class*='size-'])]:size-5"
                          >
                            <Inbox />
                          </EmptyMedia>
                          <EmptyTitle>{emptyTitle}</EmptyTitle>
                          <EmptyDescription>{emptyMessage}</EmptyDescription>
                        </EmptyHeader>
                        {emptyAction && (
                          <EmptyContent>
                            <Button size="sm" onClick={emptyAction.onClick}>
                              <Plus size={13} className="mr-1.5" />
                              {emptyAction.label}
                            </Button>
                          </EmptyContent>
                        )}
                      </Empty>
                    )}
                  </td>
                </tr>
              )}

              {/* Data rows — con reorder van dentro de un SortableContext vertical (por id de fila). */}
              {!isLoading && !isError && (
                enableRowReorder ? (
                  <SortableContext items={rows.map((r) => r.id)} strategy={verticalListSortingStrategy}>
                    {rows.map((row: Row<T>, rowIdx) => renderDataRow(row, rowIdx))}
                  </SortableContext>
                ) : (
                  rows.map((row: Row<T>, rowIdx) => renderDataRow(row, rowIdx))
                )
              )}
            </tbody>
          </table>
        </div>
      </div>
      </DndContext>

      {/* ── Pagination ── */}
      {(useServerPagination || useClientPagination) && (() => {
        const totalFiltered = useClientPagination ? table.getFilteredRowModel().rows.length : serverPagination!.total
        const currentPage   = useServerPagination ? serverPagination!.page - 1 : table.getState().pagination.pageIndex
        const effectiveSize = useServerPagination ? serverPagination!.limit : pageSize
        const totalPages    = Math.ceil(totalFiltered / effectiveSize)
        const canPrev       = currentPage > 0
        const canNext       = currentPage < totalPages - 1
        const from          = currentPage * effectiveSize + 1
        const to            = Math.min((currentPage + 1) * effectiveSize, totalFiltered)

        const goFirst = () => useServerPagination ? serverPagination!.onPageChange(1)            : table.firstPage()
        const goPrev  = () => useServerPagination ? serverPagination!.onPageChange(currentPage)  : table.previousPage()
        const goNext  = () => useServerPagination ? serverPagination!.onPageChange(currentPage + 2) : table.nextPage()
        const goLast  = () => useServerPagination ? serverPagination!.onPageChange(totalPages)   : table.lastPage()

        return (
          <div className={cn('flex items-center justify-between gap-2 text-xs text-muted-foreground flex-wrap', fillHeight && 'shrink-0')}>
            <span>{t.range(from, to, totalFiltered)}</span>
            <div className="flex items-center gap-1">
              <DropdownMenu>
                <DropdownMenuTrigger className="h-7 px-2 inline-flex items-center gap-1 rounded-md border border-input bg-background text-xs hover:bg-accent transition-colors">
                  {t.perPage(effectiveSize)} <ChevronDown size={10} />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-24">
                  {(serverPagination?.pageSizeOptions ?? PAGE_SIZES).map((size) => (
                    <DropdownMenuItem
                      key={size}
                      onClick={() => {
                        setPageSize(size)
                        serverPagination?.onLimitChange?.(size)
                        setClientPage(0)
                      }}
                      className={cn('text-xs', size === effectiveSize && 'font-medium')}
                    >
                      {t.rows(size)}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>

              {([
                { icon: ChevronsLeft,  label: t.first, disabled: !canPrev, fn: goFirst },
                { icon: ChevronLeft,   label: t.prev,  disabled: !canPrev, fn: goPrev  },
                { icon: ChevronRight,  label: t.next,  disabled: !canNext, fn: goNext  },
                { icon: ChevronsRight, label: t.last,  disabled: !canNext, fn: goLast  },
              ] as const).map(({ icon: Icon, label, disabled, fn }) => (
                <Tooltip key={label}>
                  <TooltipTrigger
                    className={cn(buttonVariants({ variant: 'outline', size: 'icon' }), 'size-7')}
                    disabled={disabled}
                    onClick={fn}
                  >
                    <Icon size={12} />
                  </TooltipTrigger>
                  <TooltipContent>{label}</TooltipContent>
                </Tooltip>
              ))}
            </div>
          </div>
        )
      })()}
    </div>
  )
}
