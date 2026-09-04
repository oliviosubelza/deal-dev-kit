---
name: web-ui
description: "The CRM DEAL shared UI catalog installed at src/shared/ui — shadcn/ui primitives on Radix + Tailwind v4, plus composed DataTable and FilterBar. Use whenever building or editing UI in crm-deal-web: forms, tables, dialogs, filters, empty and loading states, layout. Load before writing any custom markup for something this catalog already covers, and before pulling a new component in with deal-kit."
---

# CRM DEAL UI catalog

Two layers, both living under `src/shared/ui/`:

1. **Primitives** (`shared/ui/*.tsx`) — unmodified shadcn/ui components on Radix (Button, Dialog, Table, Field, Select, Sidebar, …). Standard shadcn API: if you know shadcn, you know these.
2. **Composed components** (`shared/ui/data-table/`) — built from primitives for what shadcn does not ship: `DataTable` (sortable, filterable, paginated tables with row actions) and `FilterBar` (declarative filter toolbar).

This catalog is **web only**. `crm-deal-mobile` uses React Native Paper; nothing here applies there.

Components are **copied into the project** by `deal-kit`, not imported from a package. Once installed, the project owns that code, and `deal-kit` records every file it wrote in `deal-kit.lock`.

The catalog itself knows **nothing about CRM DEAL's domain**. Never import an entity, an API client, a store, or a route into anything under `shared/ui/` — it has to keep working copied into any React + Tailwind app. If a component needs something environment-specific, add an injection point (see [Storage adapter](#storage-adapter)); do not hardcode an assumption.

## Golden rule

**You do not need raw HTML — check this catalog first.** A settings page is `Tabs` + `Card` + `Field`. A data list is `DataTable` + `defineColumns`. A confirmation is `AlertDialog`, not a custom modal. If you are about to write `<table>`, `<select>`, a styled `<span>` for a status pill, a hand-rolled empty or loading state, or any markup that mimics a component below — stop. That component exists for exactly this. If the project does not have its file yet, install it (see [Installing a component](#installing-a-component)).

Plain `<div>`s are still right for what they are for: layout glue, a flex or grid wrapper, spacing around a group. The rule is about not re-implementing a component, not about avoiding native elements.

## Installing a component

`deal-kit` resolves dependencies, installs npm packages with the project's package manager, and rewrites imports to `src/shared/ui`. Do not copy files by hand.

```sh
deal-kit add ui-kit/data-table    # pulls button, badge, checkbox, popover, … automatically
deal-kit add ui-kit/dialog
deal-kit                          # or browse the catalog interactively
```

Artifact IDs are `ui-kit/<file name without extension>`: `ui-kit/button`, `ui-kit/data-table`, `ui-kit/input-otp`. Run `deal-kit status` to see what this project already has.

Two things `deal-kit init` installs for you and everything depends on: `ui-kit/base` (which provides `cn()` and the storage adapter) and `ui-kit/theme` (`src/app/theme.css`).

## Imports

Every component is imported through the project's `@/` alias, which maps to `src/`:

```tsx
import { Button } from '@/shared/ui/button'
import { DataTable, defineColumns } from '@/shared/ui/data-table'
import { cn } from '@/shared/lib/utils'
import { useIsMobile } from '@/shared/hooks/use-mobile'
```

`src/app/theme.css` must be imported once at the app entry point, or every component renders unstyled.

## Changing a component that came from the catalog

If you are about to add a prop, change behavior, or otherwise extend a file under `shared/ui/` that `deal-kit` installed, pause before wrapping up: **tell the user this file comes from the shared kit, and ask whether the change should go back there too.**

Most improvements to a generic component belong upstream — that is the only way the rest of the team gets them. Do not ask this for components written for this project that were never part of the catalog.

Two consequences worth knowing:

- `deal-kit status` will report the file as `MODIFIED`, and `deal-kit update` will **refuse to overwrite it** rather than destroy the change. That is by design.
- Until the change is upstream, this project carries a fork of that file.

The contribution flow lives in the `deal-dev-kit` repository's `CONTRIBUTING.md`.

## Component catalog

| Need | Use |
| --- | --- |
| Button / action | `Button` (variants: default, outline, secondary, ghost, destructive) |
| Grouped buttons | `ButtonGroup` |
| Form field wrapper | `Field`, `FieldLabel`, `FieldDescription`, `FieldError`, `FieldGroup`, `FieldSet`, `FieldLegend`, `FieldSeparator`, `FieldContent`, `FieldTitle` |
| Text/number/password input | `Input` |
| Standalone label (outside a `Field`) | `Label` |
| Input with icon/button/addon | `InputGroup`, `InputGroupAddon`, `InputGroupButton`, `InputGroupText`, `InputGroupInput`, `InputGroupTextarea` |
| Multi-line text | `Textarea` |
| Select (styled) | `Select` (Radix-based, custom popover) |
| Select (native `<select>`) | `NativeSelect` — when native behavior and a11y matter more than styling |
| Searchable select / autocomplete | `Combobox` |
| OTP input | `InputOTP` |
| Checkbox / switch / radio | `Checkbox`, `Switch`, `RadioGroup` |
| Slider | `Slider` |
| 2–7 mutually exclusive options | `ToggleGroup` + `ToggleGroupItem` (never loop `Button` with manual active state) |
| Single toggle | `Toggle` |
| Date picker | `Calendar` (composes with `Popover` for a full date picker) |
| Data table (sortable/filterable/paginated) | `DataTable` — see [dedicated section](#datatable) |
| Filter toolbar above a table | `FilterBar` + `defineFilters` |
| Reusable table primitives (no logic) | `Table`, `TableHeader`, `TableBody`, `TableRow`, `TableCell`, … — only if `DataTable` genuinely does not fit |
| Card / panel | `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter` — always the full composition, never dump everything in `CardContent` |
| Badge / status pill | `Badge` (variants), never a raw styled `<span>` |
| Avatar | `Avatar` + `AvatarImage` + `AvatarFallback` (the fallback is required) |
| Tooltip | `Tooltip`, `TooltipTrigger`, `TooltipContent` |
| Hover-triggered rich preview | `HoverCard` |
| Click-triggered floating panel | `Popover` |
| Modal dialog | `Dialog` (needs `DialogTitle`; use `className="sr-only"` if visually hidden) |
| Confirmation dialog | `AlertDialog` — not a hand-rolled `Dialog` with two buttons |
| Side panel | `Sheet` |
| Bottom sheet / mobile drawer | `Drawer` |
| Command palette / fuzzy search | `Command` inside `Dialog` |
| Dropdown menu | `DropdownMenu` + `DropdownMenuGroup`/`DropdownMenuItem` (items always inside a group) |
| Right-click menu | `ContextMenu` |
| App-level menu bar | `Menubar` |
| Tabs | `Tabs` + `TabsList` + `TabsTrigger` (always inside `TabsList`) + `TabsContent` |
| Accordion (single/multi expand) | `Accordion` |
| Simple show/hide | `Collapsible` |
| Breadcrumbs | `Breadcrumb` |
| Pagination controls (standalone) | `Pagination` — `DataTable` has its own; use this only outside a table |
| Sidebar navigation | `Sidebar` (uses `useIsMobile` from `@/shared/hooks/use-mobile`) |
| Menu with submenus, mega menu | `NavigationMenu` |
| Numbered step flow | `Steps` — **custom, not stock shadcn** |
| Keyboard shortcut hint | `Kbd` — **custom** |
| Resizable panes | `ResizablePanelGroup` + `ResizablePanel` + `ResizableHandle` |
| Scrollable region with styled scrollbar | `ScrollArea` |
| Carousel | `Carousel` |
| Charts | `ChartContainer` + `ChartTooltip`/`ChartTooltipContent`/`ChartLegend`/`ChartLegendContent`, configured with the `ChartConfig` type (wraps Recharts) |
| Toast notifications | `Toaster` from `@/shared/ui/sonnet` (wraps `sonner`) — mount once at the app root, then call `toast()` from `sonner` directly |
| Empty states | `Empty`, `EmptyHeader`, `EmptyTitle`, `EmptyDescription`, `EmptyContent`, `EmptyMedia` — never hand-built |
| Callouts / inline warnings | `Alert` |
| Loading placeholder | `Skeleton` — never a custom `animate-pulse` div |
| Progress bar | `Progress` |
| Spinner | `Spinner` |
| Separator | `Separator` — never `<hr>` or a manual `border-t` div |
| Generic labeled row (list item with media/actions) | `Item`, `ItemMedia`, `ItemContent`, `ItemActions`, `ItemGroup`, `ItemSeparator`, `ItemTitle`, `ItemDescription`, `ItemHeader`, `ItemFooter` |
| Aspect-ratio box | `AspectRatio` |
| RTL support | `DirectionProvider` / `useDirection` from `@/shared/ui/direction` |
| Portal target for overlays in a secondary window | `PortalContainerContext` + `usePortalContainer` — only for OS-level secondary windows |

## Critical composition rules

Non-negotiable. Every primitive already follows them; keep following them in anything new.

- **`className` is for layout, not styling.** Never override a component's built-in colors or typography with `className`.
- **No `space-x-*`/`space-y-*`.** Use `flex` + `gap-*` (`flex flex-col gap-4` for vertical stacks).
- **`size-*` when width equals height.** `size-10`, not `w-10 h-10`.
- **`truncate`**, not `overflow-hidden text-ellipsis whitespace-nowrap`.
- **No manual `dark:` overrides.** Use semantic tokens (`bg-background`, `text-muted-foreground`, `bg-primary`, …), never raw colors like `bg-blue-500`. Dark mode comes from the token values in `theme.css`, not per-component overrides.
- **`cn()` for conditional classes**, not template-literal ternaries.
- **No manual `z-index` on overlays.** `Dialog`, `Sheet`, `Popover`, `DropdownMenu` manage their own stacking.
- **Forms use `FieldGroup` + `Field`**, never a raw `div` with `space-y-*` or `grid gap-*`.
- **`InputGroup` wraps `InputGroupInput`/`InputGroupTextarea`**, never a raw `Input`/`Textarea`.
- **Validation:** `data-invalid` on `Field`, `aria-invalid` on the control. Disabled: `data-disabled` on `Field`, `disabled` on the control.
- **Items live inside their group:** `SelectItem` → `SelectGroup`, `DropdownMenuItem` → `DropdownMenuGroup`, `CommandItem` → `CommandGroup`.
- **Icons:** pass as components (`icon={CheckIcon}`), not string lookups. Inside `Button`, put `data-icon="inline-start" | "inline-end"` on the icon and no sizing classes — the component sizes its own icons.
- **`Button` has no `isPending`/`isLoading` prop.** Compose: `<Spinner />` + `data-icon` + `disabled`.

## DataTable

Declarative, sortable, filterable, paginated, selectable table on TanStack Table + dnd-kit.

```tsx
import { DataTable, defineColumns } from '@/shared/ui/data-table'
import { Badge } from '@/shared/ui/badge'

const columns = defineColumns<Order>([
  { id: 'code', header: 'Código', accessorKey: 'code', size: 100 },
  { id: 'status', header: 'Estado', cell: (row) => <Badge>{row.status}</Badge> },
])

<DataTable tableId="orders" columns={columns} data={orders} getRowId={(o) => o.id} />
```

`tableId` must be **unique and stable** per table instance: column order, width, visibility, density, sorting and page size persist under it, keyed `data-table:v2:<tableId>`.

### Props reference

| Group | Props | Notes |
| --- | --- | --- |
| Core | `tableId`, `columns`, `data`, `getRowId?` | `getRowId` defaults to the array index. Pass it whenever rows can reorder or filter, or selection and skeleton keys drift. |
| Loading/error | `isLoading`, `isError`, `errorTitle?`, `errorMessage?`, `onRetry?` | `isLoading` shows a full skeleton body. `onRetry` turns the error state into a retry button instead of a dead end. |
| Row/cell loading | `isRowLoading?: (row) => boolean`, `isCellLoading?: (row, columnId) => boolean` | Skeleton one row or one cell without blocking the table. A cell shows a skeleton if either returns true. |
| Empty state | `emptyTitle?`, `emptyMessage?`, `emptyAction?: {label, onClick}`, `emptySlot?` | `emptyAction` turns an empty table into an actionable state. |
| Layout | `bodyMinHeight?` (default 320px), `fillHeight?` | `fillHeight` makes the table fill its flex parent — sticky header, scrolling body, pagination always visible. The parent must be a height-bounded `flex-col`. |
| Text | `labels?: Partial<DataTableLabels>` | Overrides the built-in Spanish defaults. Import `DataTableLabels` / `DEFAULT_DATA_TABLE_LABELS` to build a full translation. |
| Sort | `initialSort?: {id, desc?}` | Default sort until the user picks another column, which then persists per `tableId`. |
| Pagination | `pagination?: ServerPagination` (`page`, `limit`, `total`, `onPageChange`, `onLimitChange?`, `pageSizeOptions?`), `clientPagination?`, `defaultPageSize?` | Pick **one**: server pagination (you own the data slice) or `clientPagination` (the table slices `data`). Omit both for an unpaginated table. |
| Selection | `selectable?`, `onSelectionChange?`, `isRowSelectable?: (row) => boolean`, `defaultSelectedIds?: string[]` | `defaultSelectedIds` seeds selection **only on mount**; to reseed, remount with a different `key`. `isRowSelectable` disables a row's checkbox without hiding the row. |
| Row reorder | `enableRowReorder?`, `onRowReorder?: (activeId, overId) => void` | The table does **not** reorder `data` itself — you own the reorder on drop. |
| Actions | `rowActions?: (row) => RowAction<T>[]`, `bulkActions?: BulkAction<T>[]` | `RowAction`: `label`, `icon?`, `onClick`, `variant?: 'default' \| 'destructive'`, `disabled?`, `separator?`. `bulkActions` require `selectable`. |
| Row interaction | `onRowClick?`, `onRowDoubleClick?`, `rowClassName?: (row) => string` | |
| Expandable | `expandable?`, `renderExpanded?: (row) => ReactNode` | |
| Search | `searchable?`, `searchPlaceholder?`, `defaultSearch?`, `searchKeys?: (keyof T)[]`, `onSearchChange?` | Without `searchKeys`, search only covers columns whose value is a string or number, so a field you do not display cannot be searched. `searchKeys` searches any listed field regardless of what is rendered. |
| Export | `exportable?`, `exportFilename?` | Client-side CSV export of the visible columns and rows. |
| Slots | `filterBar?: ReactNode`, `toolbar?: ReactNode` | |
| Appearance | `defaultDensity?: DensityMode` (`'compact' \| 'normal' \| 'comfortable'`, default compact), `stickyHeader?`, `striped?` | |

### Common patterns

**Server-paginated table**, the default shape against the Gateway:

```tsx
<DataTable
  tableId="orders"
  columns={columns}
  data={page.items}
  getRowId={(o) => o.id}
  isLoading={isFetching}
  pagination={{ page, limit, total: page.total, onPageChange: setPage }}
/>
```

**Selection with bulk actions:**

```tsx
<DataTable
  tableId="orders"
  columns={columns}
  data={data}
  getRowId={(o) => o.id}
  selectable
  onSelectionChange={setSelected}
  bulkActions={[{ label: 'Anular', variant: 'destructive', onClick: (rows) => void 0 }]}
/>
```

**Skeleton a single cell while it saves**, instead of blocking the whole table:

```tsx
<DataTable
  tableId="orders"
  columns={columns}
  data={data}
  getRowId={(o) => o.id}
  isCellLoading={(order, columnId) => columnId === 'assignee' && assigningIds.has(order.id)}
/>
```

The full skeleton write-up — row versus cell, the combination rule, more examples — is in `src/shared/ui/data-table/README.md`, installed alongside the component.

## FilterBar

Declarative filter toolbar, passed into `DataTable`'s `filterBar` slot.

```tsx
import { FilterBar, defineFilters } from '@/shared/ui/data-table'

interface OrderFilters { status?: string; from?: string; to?: string; q?: string }

const filters = defineFilters<OrderFilters>([
  { type: 'select', id: 'status', label: 'Estado', options: [{ label: 'Pendiente', value: 'pending' }] },
  { type: 'daterange', id: 'range', label: 'Fecha', fromKey: 'from', toKey: 'to' },
  { type: 'text', id: 'q', label: 'Buscar', width: 'w-64' },
])

const [values, setValues] = useState<Partial<OrderFilters>>({})

<DataTable
  tableId="orders"
  columns={columns}
  data={data}
  getRowId={(o) => o.id}
  filterBar={<FilterBar defs={filters} values={values} onChange={(u) => setValues((v) => ({ ...v, ...u }))} />}
/>
```

Filter types: `text`, `select`, `asyncselect` (a `useOptions()` hook returning `{data?, isLoading?}`, for options loaded from the Gateway), `boolean`, `daterange` (drives two keys via `fromKey`/`toKey`).

`target?: 'params' | 'body'` on any filter is metadata for **your** fetch layer — whether the filter belongs in the query string or the request body. `FilterBar` ignores it; read it when building the request.

## Storage adapter

`DataTable` persists per-column state through `@/shared/lib/storage/adapter`, which defaults to `localStorage`. A plain web app needs no setup.

To swap the backing store, do it once before any `DataTable` mounts:

```ts
import { setStorageAdapter } from '@/shared/lib/storage/adapter'

setStorageAdapter({
  get: (key) => myStore.get(key),
  set: (key, value) => myStore.set(key, value),
  delete: (key) => myStore.delete(key),
})
```

This is the pattern for any environment-specific need the catalog grows: a pluggable default that works with zero configuration, plus an explicit override. Never reach into a host-specific global from inside a component.

## Theming

`src/app/theme.css` defines the semantic tokens (`--background`, `--primary`, `--radius`, …) that every component reads through Tailwind classes. To re-theme, edit the token **values**; never touch a component's classes.

**Gotcha:** tokens are stored as **HSL triplets** (`--border: 220 13% 91%`, with no `hsl()` wrapper) and consumed as `hsl(var(--border))`. A theme preset pasted in with full `hsl(...)` or `oklch(...)` strings renders **fully transparent with no error**. A previous project lost time to exactly this on the toast component. Keep new tokens in bare-triplet format.

## What NOT to do

- **Do not import CRM DEAL's domain into `shared/ui/`.** No entities, no API client, no store, no routes. That coupling is what this catalog exists to avoid. If a component needs something environment-specific, add an injection point.
- **Do not build a custom table, form layout, empty state, or skeleton** when `DataTable`, `Field`, `Empty` or `Skeleton` covers it. Check the [catalog](#component-catalog) first.
- **Do not add a barrel that re-exports everything** from `shared/ui/`. Every file has to stand on its own so `deal-kit` can install it individually; a barrel defeats that and drags the whole catalog into every bundle.
- **Do not copy component files by hand** between projects. Use `deal-kit add`, which resolves dependencies, installs packages and rewrites imports. A hand copy will show up as an unmanaged file that `deal-kit` then refuses to touch.
