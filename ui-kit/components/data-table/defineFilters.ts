import type { FilterDef } from './filter-types'

/** Helper identidad: declara filtros con inferencia sobre el modelo de filtros. */
export function defineFilters<TFilters extends Record<string, unknown>>(
  defs: FilterDef<TFilters>[],
): FilterDef<TFilters>[] {
  return defs
}
