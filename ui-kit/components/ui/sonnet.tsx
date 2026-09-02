import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { Toaster as Sonner, type ToasterProps } from "sonner"

// OJO con los tokens: este proyecto define los colores como TRIPLETAS HSL (`--popover: 0 0% 100%`), no
// como colores completos. Este componente venía de shadcn v4, que asume lo segundo, así que pasaba
// `background: var(--popover)` → `0 0% 100%`, que no es un color válido → el toast salía TRANSPARENTE. Y
// `var(--radius)` no existe en la hoja (están `--radius-lg/md/sm`) → radio inválido → esquinas cuadradas.
// Los dos síntomas venían de lo mismo, y no se habían visto nunca porque hasta ahora el <Toaster> no
// estaba montado en ningún lado del árbol. Van envueltos en `hsl()`, igual que el resto de index.css.
const Toaster = ({ ...props }: ToasterProps) => {
  return (
    <Sonner
      className="toaster group"
      // El ícono es lo ÚNICO que distingue un aviso bueno de uno malo: sin `richColors` todos los toasts
      // comparten el fondo neutro, y sin color acá los cinco íconos salían del mismo `currentColor` — o
      // sea que "Entregada" y "No entregada" se leían igual salvo por la forma del ícono a 16 px. El
      // fondo se deja neutro a propósito: así el aviso es la misma superficie que los paneles flotantes
      // del monitoreo, en vez de un bloque de color que compite con el mapa.
      icons={{
        success: <CircleCheckIcon className="size-4 text-primary" />,
        info: <InfoIcon className="size-4 text-blue-500" />,
        warning: <TriangleAlertIcon className="size-4 text-amber-500" />,
        error: <OctagonXIcon className="size-4 text-destructive" />,
        loading: <Loader2Icon className="size-4 animate-spin text-muted-foreground" />,
      }}
      style={
        {
          "--normal-bg": "hsl(var(--popover))",
          "--normal-text": "hsl(var(--popover-foreground))",
          "--normal-border": "hsl(var(--border))",
          "--border-radius": "var(--radius-lg)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast: "cn-toast",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
