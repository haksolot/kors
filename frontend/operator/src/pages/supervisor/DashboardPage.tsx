import { useEffect, useState, useCallback } from 'react'
import { RefreshCw, Activity, Bell, Monitor } from 'lucide-react'
import { fetchSupervisorDashboard } from '@/lib/api'
import type { SupervisorDashboard, WorkstationSnapshot, WorkstationStatus, Alert, ManufacturingOrder, OrderStatus, AlertCategory } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { useWebSocket } from '@/hooks/useWebSocket'

const WS_LABEL: Record<WorkstationStatus, string> = {
  WORKSTATION_STATUS_UNSPECIFIED: '—',
  WORKSTATION_STATUS_AVAILABLE: 'Disponible',
  WORKSTATION_STATUS_IN_PRODUCTION: 'En production',
  WORKSTATION_STATUS_DOWN: 'En panne',
  WORKSTATION_STATUS_MAINTENANCE: 'Maintenance',
}

function wsClass(s: WorkstationStatus): string {
  switch (s) {
    case 'WORKSTATION_STATUS_IN_PRODUCTION': return 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
    case 'WORKSTATION_STATUS_DOWN':          return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    case 'WORKSTATION_STATUS_MAINTENANCE':   return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    default:                                 return 'bg-secondary text-secondary-foreground border-0'
  }
}

const ORDER_LABEL: Record<OrderStatus, string> = {
  ORDER_STATUS_UNSPECIFIED: '—', ORDER_STATUS_PLANNED: 'Planifié',
  ORDER_STATUS_IN_PROGRESS: 'En cours', ORDER_STATUS_COMPLETED: 'Terminé',
  ORDER_STATUS_SUSPENDED: 'Suspendu', ORDER_STATUS_CANCELLED: 'Annulé',
}

function orderClass(s: OrderStatus): string {
  switch (s) {
    case 'ORDER_STATUS_IN_PROGRESS': return 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
    case 'ORDER_STATUS_PLANNED':     return 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0'
    case 'ORDER_STATUS_SUSPENDED':   return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    case 'ORDER_STATUS_CANCELLED':   return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    default:                         return 'bg-secondary text-secondary-foreground border-0'
  }
}

const ALERT_CAT_LABEL: Record<AlertCategory, string> = {
  ALERT_CATEGORY_MACHINE: 'Machine', ALERT_CATEGORY_QUALITY: 'Qualité',
  ALERT_CATEGORY_PLANNING: 'Planning', ALERT_CATEGORY_LOGISTICS: 'Logistique',
}

function alertCatClass(c: AlertCategory): string {
  switch (c) {
    case 'ALERT_CATEGORY_MACHINE':   return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    case 'ALERT_CATEGORY_QUALITY':   return 'bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300 border-0'
    case 'ALERT_CATEGORY_LOGISTICS': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    default:                         return 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0'
  }
}

function pct(v: number) { return `${Math.round(v * 100)} %` }
function trsColor(v: number) {
  if (v >= 0.85) return 'text-green-600 dark:text-green-400'
  if (v >= 0.65) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
}

function WorkstationCard({ ws }: { ws: WorkstationSnapshot }) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-2 mb-2">
          <p className="font-semibold truncate flex-1">{ws.workstation_name || ws.workstation_id}</p>
          <Badge className={wsClass(ws.status)}>{WS_LABEL[ws.status]}</Badge>
        </div>
        {ws.current_of_reference && (
          <p className="text-sm text-muted-foreground">
            OF : <span className="text-foreground font-medium">{ws.current_of_reference}</span>
          </p>
        )}
        {ws.trs != null && ws.trs > 0 && (
          <>
            <Separator className="my-2" />
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">TRS 8h</span>
              <span className={`font-bold text-sm ${trsColor(ws.trs)}`}>{pct(ws.trs)}</span>
            </div>
            <div className="flex gap-3 text-xs text-muted-foreground mt-1">
              <span>D: <span className="text-foreground">{pct(ws.availability ?? 0)}</span></span>
              <span>P: <span className="text-foreground">{pct(ws.performance ?? 0)}</span></span>
              <span>Q: <span className="text-foreground">{pct(ws.quality ?? 0)}</span></span>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}

export default function DashboardPage() {
  const [data, setData] = useState<SupervisorDashboard | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastRefresh, setLastRefresh] = useState(new Date())

  const load = useCallback(async () => {
    try {
      const d = await fetchSupervisorDashboard()
      setData(d)
      setLastRefresh(new Date())
      setError(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const id = setInterval(load, 30_000)
    return () => clearInterval(id)
  }, [load])

  useWebSocket(
    ['kors.mes.of.*', 'kors.mes.operation.*', 'kors.mes.workstation.*', 'kors.mes.alert.*'],
    useCallback(() => { load() }, [load]),
  )

  const orders      = data?.active_orders ?? []
  const workstations = data?.workstations ?? []
  const alerts      = data?.active_alerts ?? []

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">Vue d'ensemble</h1>
          <p className="text-sm text-muted-foreground">
            Mis à jour à {lastRefresh.toLocaleTimeString('fr-FR')}
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
          Actualiser
        </Button>
      </div>

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-4 text-destructive text-sm">{error}</CardContent>
        </Card>
      )}

      {loading && !data ? (
        <div className="grid grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-24" />)}
        </div>
      ) : (
        <>
          {/* KPIs */}
          <div className="grid grid-cols-3 gap-4">
            <Card>
              <CardContent className="p-4 flex flex-col items-center gap-1">
                <Activity className="size-5 text-muted-foreground" />
                <p className="text-3xl font-bold">{orders.length}</p>
                <p className="text-xs text-muted-foreground text-center">OF actifs</p>
              </CardContent>
            </Card>
            <Card className={alerts.length > 0 ? 'border-orange-300 dark:border-orange-700' : ''}>
              <CardContent className="p-4 flex flex-col items-center gap-1">
                <Bell className={`size-5 ${alerts.length > 0 ? 'text-orange-600 dark:text-orange-400' : 'text-muted-foreground'}`} />
                <p className={`text-3xl font-bold ${alerts.length > 0 ? 'text-orange-600 dark:text-orange-400' : ''}`}>
                  {alerts.length}
                </p>
                <p className="text-xs text-muted-foreground text-center">Alertes ouvertes</p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4 flex flex-col items-center gap-1">
                <Monitor className="size-5 text-muted-foreground" />
                <p className="text-3xl font-bold">
                  {workstations.filter((w) => w.status === 'WORKSTATION_STATUS_IN_PRODUCTION').length}
                  <span className="text-muted-foreground text-xl">/{workstations.length}</span>
                </p>
                <p className="text-xs text-muted-foreground text-center">Postes actifs</p>
              </CardContent>
            </Card>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Workstations */}
            <section className="flex flex-col gap-3">
              <h2 className="font-semibold">Postes de travail</h2>
              {workstations.length === 0 ? (
                <p className="text-sm text-muted-foreground">Aucun poste configuré.</p>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {workstations.map((ws) => <WorkstationCard key={ws.workstation_id} ws={ws} />)}
                </div>
              )}
            </section>

            {/* Alerts */}
            <section className="flex flex-col gap-3">
              <h2 className="font-semibold">
                Alertes actives
                {alerts.length > 0 && <span className="ml-2 text-orange-600 dark:text-orange-400">({alerts.length})</span>}
              </h2>
              <Card>
                <CardContent className="p-0">
                  {alerts.length === 0 ? (
                    <p className="text-sm text-muted-foreground p-4 text-center">Aucune alerte — atelier nominal ✓</p>
                  ) : (
                    <div className="divide-y">
                      {alerts.map((a: Alert) => (
                        <div key={a.id} className="flex items-start gap-3 p-3">
                          <Badge className={alertCatClass(a.category)}>{ALERT_CAT_LABEL[a.category]}</Badge>
                          <div className="flex-1 min-w-0">
                            <p className="text-sm">{a.message}</p>
                            {a.created_at && (
                              <p className="text-xs text-muted-foreground">
                                {new Date(a.created_at).toLocaleTimeString('fr-FR')}
                              </p>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </section>
          </div>

          {/* Active orders */}
          <section className="flex flex-col gap-3">
            <h2 className="font-semibold">OF en cours ({orders.length})</h2>
            <Card>
              <CardContent className="p-0">
                {orders.length === 0 ? (
                  <p className="text-sm text-muted-foreground p-4 text-center">Aucun OF actif.</p>
                ) : (
                  <div className="divide-y">
                    {orders.map((o: ManufacturingOrder) => (
                      <div key={o.id} className="flex items-center gap-3 p-3">
                        <div className="flex-1 min-w-0">
                          <p className="font-medium truncate">{o.reference}</p>
                          <p className="text-xs text-muted-foreground">Qté {o.quantity}</p>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          {o.is_fai && (
                            <Badge className="bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300 border-0 text-xs">FAI</Badge>
                          )}
                          <Badge className={orderClass(o.status)}>{ORDER_LABEL[o.status]}</Badge>
                          <span className="text-xs text-muted-foreground font-mono">P{o.priority}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </section>
        </>
      )}
    </div>
  )
}
