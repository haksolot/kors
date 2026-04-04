import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, AlertCircle } from 'lucide-react'
import { listCAPAs, startCAPAAction, completeCAPA } from '@/lib/api'
import type { CAPA, CAPAStatus, CAPAActionType } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const STATUS_LABEL: Record<CAPAStatus, string> = {
  CAPA_STATUS_UNSPECIFIED: '—',
  CAPA_STATUS_OPEN: 'Ouverte',
  CAPA_STATUS_IN_PROGRESS: 'En cours',
  CAPA_STATUS_COMPLETED: 'Terminée',
  CAPA_STATUS_CANCELLED: 'Annulée',
}

function statusClass(s: CAPAStatus): string {
  switch (s) {
    case 'CAPA_STATUS_OPEN':        return 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0'
    case 'CAPA_STATUS_IN_PROGRESS': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    case 'CAPA_STATUS_COMPLETED':   return 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
    default:                        return 'bg-secondary text-secondary-foreground border-0'
  }
}

const TYPE_LABEL: Record<CAPAActionType, string> = {
  CAPA_ACTION_TYPE_UNSPECIFIED: '—',
  CAPA_ACTION_TYPE_CORRECTIVE: 'Corrective',
  CAPA_ACTION_TYPE_PREVENTIVE: 'Préventive',
}

function typeClass(t: CAPAActionType): string {
  switch (t) {
    case 'CAPA_ACTION_TYPE_CORRECTIVE': return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    case 'CAPA_ACTION_TYPE_PREVENTIVE': return 'bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300 border-0'
    default:                            return 'bg-secondary text-secondary-foreground border-0'
  }
}

export default function CAPAPage() {
  const [capas, setCAPAs] = useState<CAPA[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await listCAPAs()
      setCAPAs(res.capas ?? [])
      setError(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally { setLoading(false) }
  }, [])

  useEffect(() => { load() }, [load])

  async function action(id: string, fn: () => Promise<{ capa: CAPA }>) {
    setBusy(id)
    try {
      const res = await fn()
      setCAPAs((prev) => prev.map((c) => (c.id === id ? res.capa : c)))
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally { setBusy(null) }
  }

  const active = capas.filter((c) => c.status !== 'CAPA_STATUS_COMPLETED' && c.status !== 'CAPA_STATUS_CANCELLED')
  const done   = capas.filter((c) => c.status === 'CAPA_STATUS_COMPLETED' || c.status === 'CAPA_STATUS_CANCELLED')

  const CAPATable = ({ items }: { items: CAPA[] }) => (
    <Card>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Description</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Échéance</TableHead>
            <TableHead>Statut</TableHead>
            <TableHead>Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((capa) => {
            const overdue = capa.due_date && new Date(capa.due_date) < new Date() && capa.status !== 'CAPA_STATUS_COMPLETED'
            return (
              <TableRow key={capa.id}>
                <TableCell>
                  <p className="font-medium">{capa.description ?? 'Sans description'}</p>
                  <p className="text-xs text-muted-foreground font-mono">NC : {capa.nc_id.slice(0, 8)}…</p>
                </TableCell>
                <TableCell>
                  <Badge className={typeClass(capa.action_type)}>{TYPE_LABEL[capa.action_type]}</Badge>
                </TableCell>
                <TableCell className={`text-sm ${overdue ? 'text-red-600 dark:text-red-400 font-medium' : 'text-muted-foreground'}`}>
                  {capa.due_date ? new Date(capa.due_date).toLocaleDateString('fr-FR') : '—'}
                </TableCell>
                <TableCell>
                  <Badge className={statusClass(capa.status)}>{STATUS_LABEL[capa.status]}</Badge>
                </TableCell>
                <TableCell>
                  <div className="flex gap-2">
                    {capa.status === 'CAPA_STATUS_OPEN' && (
                      <Button variant="outline" size="sm" onClick={() => action(capa.id, () => startCAPAAction(capa.id))} disabled={busy === capa.id}>
                        Démarrer
                      </Button>
                    )}
                    {capa.status === 'CAPA_STATUS_IN_PROGRESS' && (
                      <Button variant="outline" size="sm" className="text-green-700 dark:text-green-400 border-green-300 dark:border-green-700" onClick={() => action(capa.id, () => completeCAPA(capa.id))} disabled={busy === capa.id}>
                        Terminer
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </Card>
  )

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">
            CAPA
            {active.length > 0 && <span className="ml-2 text-yellow-600 dark:text-yellow-400">({active.length} en cours)</span>}
          </h1>
          <p className="text-sm text-muted-foreground">Actions correctives et préventives</p>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-3 text-destructive text-sm flex items-center gap-2">
            <AlertCircle className="size-4 shrink-0" />{error}
          </CardContent>
        </Card>
      )}

      {loading ? (
        <Skeleton className="h-48 w-full" />
      ) : capas.length === 0 ? (
        <Card><CardContent className="p-8 text-center text-muted-foreground">Aucune CAPA enregistrée.</CardContent></Card>
      ) : (
        <Tabs defaultValue="active">
          <TabsList>
            <TabsTrigger value="active">
              En cours
              {active.length > 0 && <Badge variant="secondary" className="ml-1.5 px-1.5 py-0 text-xs">{active.length}</Badge>}
            </TabsTrigger>
            <TabsTrigger value="done">Terminées ({done.length})</TabsTrigger>
          </TabsList>
          <TabsContent value="active">
            {active.length === 0
              ? <Card><CardContent className="p-6 text-center text-muted-foreground">Aucune CAPA active.</CardContent></Card>
              : <CAPATable items={active} />}
          </TabsContent>
          <TabsContent value="done">
            {done.length === 0
              ? <Card><CardContent className="p-6 text-center text-muted-foreground">Aucune CAPA clôturée.</CardContent></Card>
              : <CAPATable items={done} />}
          </TabsContent>
        </Tabs>
      )}
    </div>
  )
}
