import { Check, type LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface StepItem {
  id: string
  label: string
  /** Línea secundaria bajo el label (opcional): estado, conteo, ayuda corta. */
  description?: string
  icon?: LucideIcon
  /** Paso deshabilitado: no navegable y atenuado (ej. una fase todavía no disponible). */
  disabled?: boolean
}

interface StepsProps {
  steps: StepItem[]
  /** Índice 0-based del paso activo. Los anteriores se consideran completados. */
  current: number
  /** Si se pasa, los pasos ya completados (y el actual) se vuelven clickeables para volver atrás. */
  onStepClick?: (index: number, step: StepItem) => void
  className?: string
}

/**
 * Indicador de fases de un flujo (wizard). El paso actual va en color de marca, los completados
 * en verde tenue con tilde y los pendientes en gris; el conector entre dos pasos se "llena" cuando
 * el tramo ya fue superado, así el avance se lee de un vistazo sin contar círculos.
 */
export function Steps({ steps, current, onStepClick, className }: StepsProps) {
  return (
    <nav aria-label="Progreso" className={cn('flex w-full items-start', className)}>
      {steps.map((step, i) => {
        const isCompleted = i < current
        const isCurrent = i === current
        // Volver atrás sí; saltear pasos no hechos, no; los deshabilitados nunca.
        const isNavigable = !!onStepClick && i <= current && !step.disabled
        const Icon = step.icon

        return (
          <div key={step.id} className="flex flex-1 items-start last:flex-none">
            <button
              type="button"
              disabled={!isNavigable}
              onClick={() => onStepClick?.(i, step)}
              aria-current={isCurrent ? 'step' : undefined}
              title={step.disabled ? 'Próximamente' : undefined}
              className={cn(
                'group flex shrink-0 items-center gap-1.5 rounded-md px-1.5 py-0.5 text-left transition-colors',
                isNavigable ? 'cursor-pointer hover:bg-accent/60' : 'cursor-default',
                step.disabled && 'opacity-40'
              )}
            >
              <span
                className={cn(
                  'flex size-6 shrink-0 items-center justify-center rounded-full border transition-all',
                  isCurrent && 'border-primary bg-primary text-primary-foreground ring-2 ring-primary/15',
                  isCompleted && 'border-primary/30 bg-primary/10 text-primary',
                  !isCurrent && !isCompleted && 'border-border bg-muted text-muted-foreground'
                )}
              >
                {isCompleted ? (
                  <Check size={12} strokeWidth={2.5} />
                ) : Icon ? (
                  <Icon size={12} />
                ) : (
                  <span className="text-[10px] font-semibold tabular-nums">{i + 1}</span>
                )}
              </span>

              <span className="flex min-w-0 flex-col">
                <span
                  className={cn(
                    'truncate text-xs leading-tight',
                    isCurrent && 'font-semibold text-foreground',
                    isCompleted && 'font-medium text-foreground/80',
                    !isCurrent && !isCompleted && 'text-muted-foreground'
                  )}
                >
                  {step.label}
                </span>
                {step.description && (
                  <span className="truncate text-[10px] leading-tight text-muted-foreground">
                    {step.description}
                  </span>
                )}
              </span>
            </button>

            {/* Conector: vive dentro del paso (no como hermano) para que el `flex-1` reparta el
                sobrante entre los tramos y no entre las etiquetas, que quedan a su ancho natural. */}
            {i < steps.length - 1 && (
              <span
                aria-hidden
                className={cn(
                  'mx-1.5 mt-3.5 h-px min-w-4 flex-1 rounded-full transition-colors',
                  isCompleted ? 'bg-primary/40' : 'bg-border'
                )}
              />
            )}
          </div>
        )
      })}
    </nav>
  )
}
