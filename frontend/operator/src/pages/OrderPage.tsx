import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, CheckCircle2, Circle, Clock, PlayCircle, AlertCircle } from 'lucide-react'
import { fetchOrder, fetchOperations } from '@/lib/api'
import type { ManufacturingOrder, Operation, OperationStatus } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Separator } from '@/components/ui/separator'

const OP_STATUS_LABEL: Record<OperationStatus, string> = {
  OPERATION_STATUS_UNSPECIFIED: '—',
  OPERATION_STATUS_PENDING: 'En attente',
  OPERATION_STATUS_IN_PROGRESS: 'En cours',
  OPERATION_STATUS_COMPLETED: 'Terminée',
  OPERATION_STATUS_SKIPPED: 'Ignorée',
  OPERATION_STATUS_PENDING_SIGN_OFF: 'Attente visa',
  OPERATION_STATUS_RELEASED: 'Validée',
}

function opStatusClass(s: OperationStatus): string {
  switch (s) {
    case 'OPERATION_STATUS_IN_PROGRESS':      return 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
    case 'OPERATION_STATUS_COMPLETED':
    case 'OPERATION_STATUS_RELEASED':         return 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0'
    case 'OPERATION_STATUS_PENDING_SIGN_OFF': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    case 'OPERATION_STATUS_SKIPPED':          return 'bg-secondary text-secondary-foreground border-0'
    default:                                  return 'bg-secondary text-secondary-foreground border-0'
  }
}

function StepIcon({ status }: { status: OperationStatus }) {
  if (status === 'OPERATION_STATUS_COMPLETED' || status === 'OPERATION_STATUS_RELEASED') {
    return <CheckCircle2 className="size-5 text-blue-600 dark:text-blue-400" />
  }
  if (status === 'OPERATION_STATUS_IN_PROGRESS') {
    return <PlayCircle className="size-5 text-green-600 dark:text-green-400" />
  }
  if (status === 'OPERATION_STATUS_PENDING_SIGN_OFF') {
    return <Clock className="size-5 text-yellow-600 dark:text-yellow-400" />
  }
  return <Circle className="size-5 text-muted-foreground" />
}

function isActionable(op: Operation) {
  return op.status === 'OPERATION_STATUS_PENDING' || op.status === 'OPERATION_STATUS_IN_PROGRESS'
}

export default function OrderPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [order, setOrder] = useState<ManufacturingOrder | null>(null)
  const [operations, setOperations] = useState<Operation[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    Promise.all([fetchOrder(id), fetchOperations(id)])
      .then(([orderRes, opsRes]) => {
        setOrder(orderRes.order)
        setOperations([...(opsRes.operations ?? [])].sort((a, b) => a.step_number - b.step_number))
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Erreur'))
      .finally(() => setLoading(false))
  }, [id])

  if (loading) {
    return (
      <div className="flex flex-col gap-4 max-w-2xl mx-auto">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-32 w-full" />
        {[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}
      </div>
    )
  }

  if (error || !order) {
    return (
      <div className="max-w-2xl mx-auto flex flex-col gap-4">
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertDescription>{error ?? 'OF introuvable'}</AlertDescription>
        </Alert>
        <Button variant="outline" onClick={() => navigate('/dispatch')}>
          <ArrowLeft className="size-4" />
          Retour aux OF
        </Button>
      </div>
    )
  }

  const currentOp =
    operations.find((op) => op.status === 'OPERATION_STATUS_IN_PROGRESS') ??
    operations.find((op) => op.status === 'OPERATION_STATUS_PENDING')

  const completed = operations.filter(
    (op) => op.status === 'OPERATION_STATUS_COMPLETED' || op.status === 'OPERATION_STATUS_RELEASED',
  ).length

  return (
    <div className="flex flex-col gap-4 max-w-2xl mx-auto">
      {/* Back + title */}
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate('/dispatch')}>
          <ArrowLeft className="size-5" />
        </Button>
        <div className="flex-1 min-w-0">
          <h1 className="text-xl font-bold truncate">{order.reference}</h1>
          <p className="text-sm text-muted-foreground">
            Qté&nbsp;{order.quantity}
            {order.due_date && (
              <> · Échéance&nbsp;{new Date(order.due_date).toLocaleDateString('fr-FR')}</>
            )}
            {' '}· {completed}/{operations.length} op.
          </p>
        </div>
        {order.is_fai && (
          <Badge className="bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300 border-0">
            FAI
          </Badge>
        )}
      </div>

      {/* Current operation highlight */}
      {currentOp && (
        <Card className="border-green-300 dark:border-green-800 bg-green-50 dark:bg-green-950/30">
          <CardContent className="p-4 flex flex-col gap-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-green-700 dark:text-green-400">
              Opération courante — étape {currentOp.step_number}
            </p>
            <p className="font-bold text-lg">{currentOp.name}</p>
            <Button
              size="lg"
              className="w-full bg-green-600 hover:bg-green-700 dark:bg-green-700 dark:hover:bg-green-600 text-white"
              onClick={() => navigate(`/order/${order.id}/operation/${currentOp.id}`)}
            >
              <PlayCircle className="size-5" />
              Ouvrir l'opération
            </Button>
          </CardContent>
        </Card>
      )}

      <Separator />

      {/* Operations list */}
      <div>
        <p className="text-sm font-medium text-muted-foreground mb-3">
          Toutes les opérations ({operations.length})
        </p>
        <div className="flex flex-col gap-2">
          {operations.map((op) => (
            <Card
              key={op.id}
              className={isActionable(op) ? 'cursor-pointer hover:bg-accent transition-colors' : 'opacity-60'}
              onClick={() => { if (isActionable(op)) navigate(`/order/${order.id}/operation/${op.id}`) }}
            >
              <CardContent className="px-4 py-3 flex items-center gap-3">
                <StepIcon status={op.status} />
                <div className="flex-1 min-w-0">
                  <p className="font-medium truncate">{op.name}</p>
                  {op.planned_duration_seconds != null && op.planned_duration_seconds > 0 && (
                    <p className="text-xs text-muted-foreground">
                      {Math.round(op.planned_duration_seconds / 60)} min alloués
                    </p>
                  )}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <span className="text-xs text-muted-foreground font-mono">
                    #{op.step_number}
                  </span>
                  <Badge className={opStatusClass(op.status)}>
                    {OP_STATUS_LABEL[op.status]}
                  </Badge>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </div>
  )
}
