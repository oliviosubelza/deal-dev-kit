export interface StorageAdapter {
  get<T>(key: string): Promise<T | null>
  set<T>(key: string, value: T): Promise<void>
  delete(key: string): Promise<void>
}

const localStorageAdapter: StorageAdapter = {
  async get<T>(key: string): Promise<T | null> {
    try {
      const raw = window.localStorage.getItem(key)
      return raw ? (JSON.parse(raw) as T) : null
    } catch {
      return null
    }
  },
  async set<T>(key: string, value: T): Promise<void> {
    window.localStorage.setItem(key, JSON.stringify(value))
  },
  async delete(key: string): Promise<void> {
    window.localStorage.removeItem(key)
  },
}

let activeAdapter: StorageAdapter = localStorageAdapter

/**
 * Reemplaza el adaptador de persistencia usado por los componentes (ej. estado de columnas del
 * DataTable). Por defecto usa `localStorage`. Llamalo una vez al arrancar la app si tu host tiene
 * su propio almacenamiento (ej. `window.electron.storage` en una app Electron).
 */
export function setStorageAdapter(adapter: StorageAdapter): void {
  activeAdapter = adapter
}

export const storage: StorageAdapter = {
  get: (key) => activeAdapter.get(key),
  set: (key, value) => activeAdapter.set(key, value),
  delete: (key) => activeAdapter.delete(key),
}
