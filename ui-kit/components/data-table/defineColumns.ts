import type { ColumnDefConfig } from './types'

/** Helper identidad: declara columnas con inferencia de tipos sobre la fila. */
export function defineColumns<T extends object>(columns: ColumnDefConfig<T>[]): ColumnDefConfig<T>[] {
  return columns
}
