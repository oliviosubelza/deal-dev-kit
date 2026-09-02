import { useState, useEffect, useRef } from 'react'
import { CalendarDays, X, Plus } from 'lucide-react'
import type { DateRange } from 'react-day-picker'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { Calendar } from '@/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import type {
  FilterDef,
  FilterOption,
  TextFilterDef,
  AsyncSelectFilterDef,
  BooleanFilterDef,
  DateRangeFilterDef,
} from './filter-types'

const addFilterClass =
  'h-7 inline-flex items-center gap-1 rounded-full border border-dashed px-2.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground hover:border-primary/40 transition-colors'

function FilterChip({
  label,
  value,
  onClear,
}: {
  label: string
  value: string
  onClear: () => void
}) {
  return (
    <div className="h-7 inline-flex items-center gap-1.5 rounded-full border border-primary/30 bg-primary/8 pl-2.5 pr-1 text-xs">
      <span className="text-muted-foreground">{label}:</span>
      <span className="font-medium text-foreground">{value}</span>
      <button
        onClick={onClear}
        className="inline-flex items-center justify-center w-4 h-4 rounded-full hover:bg-primary/20 text-muted-foreground hover:text-foreground"
      >
        <X size={10} />
      </button>
    </div>
  )
}

function SelectChipFilter({
  label,
  placeholder,
  options,
  value,
  isLoading,
  onChange,
}: {
  label: string
  placeholder?: string
  options: FilterOption[]
  value: string | undefined
  isLoading?: boolean
  onChange: (value: string | undefined) => void
}) {
  const [open, setOpen] = useState(false)
  const selectedLabel = options.find((o) => o.value === value)?.label

  if (value) {
    return (
      <FilterChip
        label={label}
        value={selectedLabel ?? value}
        onClear={() => onChange(undefined)}
      />
    )
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger className={addFilterClass}>
        <Plus size={11} />
        {label}
      </PopoverTrigger>
      <PopoverContent className="p-0 w-48" align="start" side="bottom">
        {isLoading ? (
          <div className="p-2 space-y-1.5">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-5/6" />
          </div>
        ) : (
          <Command>
            <CommandInput
              placeholder={placeholder ?? `Buscar ${label.toLowerCase()}...`}
              className="h-8 text-xs"
            />
            <CommandList>
              <CommandEmpty className="text-xs py-4 text-center text-muted-foreground">
                Sin resultados
              </CommandEmpty>
              <CommandGroup>
                {options.map((opt) => (
                  <CommandItem
                    key={opt.value}
                    value={opt.label}
                    onSelect={() => { onChange(opt.value); setOpen(false) }}
                    className="text-xs cursor-pointer"
                  >
                    {opt.label}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        )}
      </PopoverContent>
    </Popover>
  )
}

function AsyncSelectFilter<TFilters extends Record<string, unknown>>({
  def,
  value,
  onChange,
}: {
  def: AsyncSelectFilterDef<TFilters>
  value: string | undefined
  onChange: (value: string | undefined) => void
}) {
  const { data: options = [], isLoading } = def.useOptions()
  return (
    <SelectChipFilter
      label={def.label}
      placeholder={def.placeholder}
      options={options}
      value={value}
      isLoading={isLoading}
      onChange={onChange}
    />
  )
}

function TextFilter<TFilters extends Record<string, unknown>>({
  def,
  value,
  onChange,
}: {
  def: TextFilterDef<TFilters>
  value: string | undefined
  onChange: (value: string | undefined) => void
}) {
  const [local, setLocal] = useState(value ?? '')
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => { setLocal(value ?? '') }, [value])

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const v = e.target.value
    setLocal(v)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => onChange(v || undefined), 300)
  }

  return (
    <div className="relative">
      <Input
        value={local}
        onChange={handleChange}
        placeholder={def.placeholder ?? def.label}
        className={cn('h-7 text-xs w-36 pr-6', def.width)}
      />
      {local && (
        <button
          className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          onClick={() => { setLocal(''); onChange(undefined) }}
        >
          <X size={11} />
        </button>
      )}
    </div>
  )
}

function DateRangeFilter<TFilters extends Record<string, unknown>>({
  def,
  fromValue,
  toValue,
  onChange,
}: {
  def: DateRangeFilterDef<TFilters>
  fromValue: string | undefined
  toValue: string | undefined
  onChange: (update: Partial<TFilters>) => void
}) {
  // ISO ("2026-07-13T…") ↔ Date local, sin corrimiento por zona horaria (se usa solo la parte fecha).
  const parse = (iso: string | undefined): Date | undefined => {
    if (!iso) return undefined
    const [y, m, d] = iso.slice(0, 10).split('-').map(Number)
    return new Date(y, m - 1, d)
  }
  const ymd = (date: Date) =>
    `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
  const short = (date: Date) =>
    date.toLocaleDateString('es-AR', { day: '2-digit', month: '2-digit' })

  const from = parse(fromValue)
  const to = parse(toValue)
  const hasAny = !!(from || to)
  const range: DateRange | undefined = from ? { from, to } : undefined

  const label =
    from && to ? `${short(from)} — ${short(to)}` : from ? `Desde ${short(from)}` : def.label

  const apply = (r: DateRange | undefined) =>
    onChange({
      [def.fromKey]: r?.from ? `${ymd(r.from)}T00:00:00.000Z` : undefined,
      [def.toKey]: r?.to ? `${ymd(r.to)}T23:59:59.999Z` : undefined,
    } as Partial<TFilters>)

  const clear = () =>
    onChange({ [def.fromKey]: undefined, [def.toKey]: undefined } as Partial<TFilters>)

  return (
    <Popover>
      <PopoverTrigger
        className={cn(
          'h-7 inline-flex items-center gap-1.5 rounded-full border pl-2.5 text-xs transition-colors',
          hasAny
            ? 'border-primary/30 bg-primary/8 pr-1 text-foreground'
            : 'border-input pr-2.5 text-muted-foreground hover:bg-accent hover:text-foreground'
        )}
      >
        <CalendarDays size={12} className="text-muted-foreground" />
        <span>{label}</span>
        {hasAny && (
          <span
            role="button"
            tabIndex={-1}
            aria-label="Limpiar fecha"
            className="ml-0.5 inline-flex size-4 items-center justify-center rounded-full text-muted-foreground hover:bg-primary/20 hover:text-foreground"
            onClick={(e) => {
              e.stopPropagation()
              clear()
            }}
          >
            <X size={11} />
          </span>
        )}
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <Calendar mode="range" numberOfMonths={2} selected={range} onSelect={apply} autoFocus />
      </PopoverContent>
    </Popover>
  )
}

function BooleanFilter<TFilters extends Record<string, unknown>>({
  def,
  value,
  onChange,
}: {
  def: BooleanFilterDef<TFilters>
  value: boolean | undefined
  onChange: (value: boolean | undefined) => void
}) {
  const id = `filter-bool-${def.id}`
  const active = value ?? false
  // Chip bordeado: visible apagado (borde + fondo tenue) y con color de acento encendido.
  return (
    <label
      htmlFor={id}
      className={cn(
        'flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs cursor-pointer select-none transition-colors',
        active
          ? 'border-primary/50 bg-primary/10 text-primary'
          : 'border-border bg-muted/40 text-muted-foreground hover:bg-accent hover:text-foreground',
      )}
    >
      <Switch
        id={id}
        checked={active}
        onCheckedChange={(checked) => onChange(checked || undefined)}
        className="scale-75 origin-left"
      />
      {def.label}
    </label>
  )
}

export function FilterBar<TFilters extends Record<string, unknown>>({
  defs,
  values,
  onChange,
}: {
  defs: FilterDef<TFilters>[]
  values: Partial<TFilters>
  onChange: (update: Partial<TFilters>) => void
}) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      {defs.map((def) => {
        if (def.type === 'text') {
          return (
            <TextFilter
              key={def.id}
              def={def}
              value={values[def.id] as string | undefined}
              onChange={(v) => onChange({ [def.id]: v } as Partial<TFilters>)}
            />
          )
        }

        if (def.type === 'select') {
          return (
            <SelectChipFilter
              key={def.id}
              label={def.label}
              placeholder={def.placeholder}
              options={def.options}
              value={values[def.id] as string | undefined}
              onChange={(v) => onChange({ [def.id]: v } as Partial<TFilters>)}
            />
          )
        }

        if (def.type === 'asyncselect') {
          return (
            <AsyncSelectFilter
              key={def.id}
              def={def}
              value={values[def.id] as string | undefined}
              onChange={(v) => onChange({ [def.id]: v } as Partial<TFilters>)}
            />
          )
        }

        if (def.type === 'daterange') {
          return (
            <DateRangeFilter
              key={def.id}
              def={def}
              fromValue={values[def.fromKey] as string | undefined}
              toValue={values[def.toKey] as string | undefined}
              onChange={onChange}
            />
          )
        }

        if (def.type === 'boolean') {
          return (
            <BooleanFilter
              key={def.id}
              def={def}
              value={values[def.id] as boolean | undefined}
              onChange={(v) => onChange({ [def.id]: v } as Partial<TFilters>)}
            />
          )
        }

        return null
      })}
    </div>
  )
}
