import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { RefreshCw, ChevronRight, AlertCircle } from 'lucide-react'
import { fetchDispatch } from '@/lib/api'
import type { ManufacturingOrder, OrderStatus } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert, AlertDescription } from '@/components/ui/alert'

const STATUS_LABEL: Record<OrderStatus, string> = {
  ORDER_STATUS_UNSPECIFIED: '—',
  ORDER_STATUS_PLANNED: 'Planifié',
  ORDER_STATUS_IN_PROGRESS: 'En cours',
  ORDER_STATUS_COMPLETED: 'Terminé',
  ORDER_STATUS_SUSPENDED: 'Suspendu',
  ORDER_STATUS_CANCELLED: 'Annulé',
}

function statusClass(s: OrderStatus): string {
  switch (s) {
    case 'ORDER_STATUS_IN_PROGRESS': return 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
    case 'ORDER_STATUS_PLANNED':     return 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0'
    case 'ORDER_STATUS_SUSPENDED':   return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    case 'ORDER_STATUS_CANCELLED':   return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    default: return 'bg-secondary text-secondary-foreground border-0'
  }
}

function priorityClass(p: number): string {
  if (p >= 80) return 'text-red-600 dark:text-red-400'
  if (p >= 50) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-muted-foreground'
}

export default function DispatchPage() {
  const [orders, setOrders] = useState<ManufacturingOrder[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const data = await fetchDispatch(50)
      setOrders(data.orders ?? [])
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Erreur réseau'
      if (msg.includes('401') || msg.includes('unauthorized')) {
        localStorage.removeItem('jwt_token')
        navigate('/login')
        return
      }
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  return (
    <div className="flex flex-col gap-4 max-w-2xl mx-auto">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">Mes OF assignés</h1>
          <p className="text-sm text-muted-foreground">Ordres de fabrication de ce poste</p>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
          Actualiser
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {loading ? (
        <div className="flex flex-col gap-3">
          {[1, 2, 3].map((i) => (
            <Card key={i}><CardContent className="p-4"><Skeleton className="h-14 w-full" /></CardContent></Card>
          ))}
        </div>
      ) : orders.length === 0 ? (
        <Card>
          <CardContent className="p-8 text-center text-muted-foreground">
            Aucun OF assigné à ce poste.
          </CardContent>
        </Card>
      ) : (
        <div className="flex flex-col gap-2">
          {orders.map((order) => (
            <Card
              key={order.id}
              className="cursor-pointer hover:bg-accent transition-colors"
              onClick={() => navigate(`/order/${order.id}`)}
            >
              <CardContent className="p-4">
                <div className="flex items-center gap-3">
                  <div className="flex-1 min-w-0">
                    <p className="font-bold text-base truncate">{order.reference}</p>
                    <div className="flex items-center gap-3 mt-0.5 text-sm text-muted-foreground">
                      <span>Qté&nbsp;<span className="text-foreground font-medium">{order.quantity}</span></span>
                      {order.due_date && (
                        <span>Échéance&nbsp;
                          <span className="text-foreground">
                            {new Date(order.due_date).toLocaleDateString('fr-FR')}
                          </span>
                        </span>
                      )}
                    </div>
                    {order.is_fai && (
                      <Badge className="mt-1 bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300 border-0 text-xs">
                        FAI
                      </Badge>
                    )}
                  </div>
                  <div className="flex flex-col items-end gap-1.5 shrink-0">
                    <Badge className={statusClass(order.status)}>
                      {STATUS_LABEL[order.status]}
                    </Badge>
                    <span className={`text-xs font-mono font-bold ${priorityClass(order.priority)}`}>
                      P{order.priority}
                    </span>
                  </div>
                  <ChevronRight className="size-4 text-muted-foreground shrink-0" />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
