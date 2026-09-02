import { createContext, useContext } from 'react'

/**
 * Contenedor donde los overlays (menús, popovers, selects, dialogs, tooltips) deben
 * portalizarse. `null` = `document.body` del documento principal (default de Base UI).
 *
 * En VENTANAS AUXILIARES (WindowPortal) el árbol React vive en el renderer principal pero se
 * pinta en otra ventana: sin esto, los portales caen en el body PRINCIPAL → aparecen detrás de
 * la ventana auxiliar y descolocados respecto del trigger. WindowPortal provee aquí el `body`
 * de la ventana hija para que los overlays se rendericen donde corresponde.
 */
// `undefined` (no null) = default de Base UI (document.body del doc principal). Pasar `null`
// explícito a `container` rompe el Portal de Base UI (no renderiza).
export const PortalContainerContext = createContext<HTMLElement | undefined>(undefined)

export const usePortalContainer = () => useContext(PortalContainerContext)
