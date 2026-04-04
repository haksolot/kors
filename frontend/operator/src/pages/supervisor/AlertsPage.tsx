import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, CheckCircle2, AlertCircle } from 'lucide-react'
import { fetchSupervisorDashboard, acknowledgeAlert, resolveAlert } from '@/lib/api'
import type { Alert, AlertCategory } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useWebSocket } from '@/hooks/useWebSocket'

const CAT_LABEL: Record<AlertCategory, string> = {
  ALERT_CATEGORY_MACHINE: 'Machine',
  ALERT_CATEGORY_QUALITY: 'Qualité',
  ALERT_CATEGORY_PLANNING: 'Planning',
  ALERT_CATEGORY_LOGISTICS: 'Logistique',
}

function catClass(c: AlertCategory): string {
  switch (c) {
    case 'ALERT_CATEGORY_MACHINE':   return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    case 'ALERT_CATEGORY_QUALITY':   return 'bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300 border-0'
    case 'ALERT_CATEGORY_LOGISTICS': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    default:                         return 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0'
  }
}

export default function AlertsPage() {
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [resolveId, setResolveId] = useState<string | null>(null)
  const [resolveNotes, setResolveNotes] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const d = await fetchSupervisorDashboard()
      setAlerts(d.active_alerts ?? [])
      setError(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])
  useWebSocket(['kors.mes.alert.*'], useCallback(() => { load() }, [load]))

  async function handleAcknowledge(id: string) {
    if (busy) return
    setBusy(true)
    try { await acknowledgeAlert(id); await load() }
    catch (err: unknown) { setError(err instanceof Error ? err.message : 'Erreur') }
    finally { setBusy(false) }
  }

  async function handleResolve() {
    if (!resolveId || busy) return
    setBusy(true)
    try {
      await resolveAlert(resolveId, resolveNotes)
      setResolveId(null)
      setResolveNotes('')
      await load()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally { setBusy(false) }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">
            Alertes actives
            {alerts.length > 0 && (
              <span className="ml-2 text-orange-600 dark:text-orange-400">({alerts.length})</span>
            )}
          </h1>
          <p className="text-sm text-muted-foreground">Gestion des aléas en cours</p>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
          Actualiser
        </Button>
      </div>

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-3 text-destructive text-sm flex items-center gap-2">
            <AlertCircle className="size-4 shrink-0" />
            {error}
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="flex flex-col gap-3">
          {[1, 2].map((i) => <Skeleton key={i} className="h-24 w-full" />)}
        </div>
      ) : alerts.length === 0 ? (
        <Card>
          <CardContent className="p-8 text-center text-muted-foreground flex flex-col items-center gap-2">
            <CheckCircle2 className="size-8 text-green-600 dark:text-green-400" />
            Aucune alerte active — atelier nominal
          </CardContent>
        </Card>
      ) : (
        <div className="flex flex-col gap-3">
          {alerts.map((alert) => (
            <Card key={alert.id}>
              <CardContent className="p-4">
                <div className="flex items-start gap-3">
                  <Badge className={catClass(alert.category)}>{CAT_LABEL[alert.category]}</Badge>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium">{alert.message}</p>
                    {alert.created_at && (
                      <p className="text-xs text-muted-foreground mt-0.5">
                        {new Date(alert.created_at).toLocaleString('fr-FR')}
                      </p>
                    )}
                  </div>
                </div>
                <div className="flex gap-2 mt-3">
                  {alert.status !== 'ALERT_STATUS_ACKNOWLEDGED' && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleAcknowledge(alert.id)}
                      disabled={busy}
                    >
                      Accuser réception
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="outline"
                    className="text-green-700 dark:text-green-400 border-green-300 dark:border-green-700 hover:bg-green-50 dark:hover:bg-green-950/30"
                    onClick={() => { setResolveId(alert.id); setResolveNotes('') }}
                    disabled={busy}
                  >
                    <CheckCircle2 className="size-4" />
                    Clôturer
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={!!resolveId} onOpenChange={(open) => { if (!open) setResolveId(null) }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Clôturer l'alerte</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="notes">Action réalisée (optionnel)</Label>
            <Textarea
              id="notes"
              value={resolveNotes}
              onChange={(e) => setResolveNotes(e.target.value)}
              placeholder="Décrivez l'action corrective…"
              rows={3}
            />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setResolveId(null)}>Annuler</Button>
            <Button onClick={handleResolve} disabled={busy}>
              Confirmer la clôture
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
