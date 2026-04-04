import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, AlertCircle, Wrench } from 'lucide-react'
import { listTools, calibrateTool } from '@/lib/api'
import type { Tool, ToolStatus } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const STATUS_LABEL: Record<ToolStatus, string> = {
  TOOL_STATUS_UNSPECIFIED: '—',
  TOOL_STATUS_VALID: 'Valide',
  TOOL_STATUS_EXPIRED: 'Étalonnage expiré',
  TOOL_STATUS_BLOCKED: 'Bloqué',
  TOOL_STATUS_DECOMMISSIONED: 'Retiré',
}

function statusClass(s: ToolStatus): string {
  switch (s) {
    case 'TOOL_STATUS_VALID':          return 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
    case 'TOOL_STATUS_EXPIRED':        return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    case 'TOOL_STATUS_BLOCKED':        return 'bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300 border-0'
    case 'TOOL_STATUS_DECOMMISSIONED': return 'bg-secondary text-secondary-foreground border-0'
    default:                           return 'bg-secondary text-secondary-foreground border-0'
  }
}

function daysUntil(dateStr?: string): number | null {
  if (!dateStr) return null
  return Math.floor((new Date(dateStr).getTime() - Date.now()) / 86400000)
}

type FilterMode = 'all' | 'expired' | 'soon'

export default function ToolsPage() {
  const [tools, setTools] = useState<Tool[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [filter, setFilter] = useState<FilterMode>('all')
  const [calibrateId, setCalibrateId] = useState<string | null>(null)
  const [lastCal, setLastCal] = useState(new Date().toISOString().slice(0, 10))
  const [nextCal, setNextCal] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await listTools()
      setTools(res.tools ?? [])
      setError(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally { setLoading(false) }
  }, [])

  useEffect(() => { load() }, [load])

  async function handleCalibrate() {
    if (!calibrateId || !lastCal || !nextCal || busy) return
    setBusy(calibrateId)
    try {
      const res = await calibrateTool(calibrateId, `${lastCal}T00:00:00Z`, `${nextCal}T00:00:00Z`)
      setTools((prev) => prev.map((t) => (t.id === calibrateId ? res.tool : t)))
      setCalibrateId(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Erreur lors de l'étalonnage")
    } finally { setBusy(null) }
  }

  const expiredCount = tools.filter((t) => t.status === 'TOOL_STATUS_EXPIRED').length
  const soonCount    = tools.filter((t) => { const d = daysUntil(t.next_calibration_at); return d !== null && d >= 0 && d <= 30 }).length

  const displayed = tools.filter((t) => {
    if (filter === 'expired') return t.status === 'TOOL_STATUS_EXPIRED'
    if (filter === 'soon') { const d = daysUntil(t.next_calibration_at); return d !== null && d >= 0 && d <= 30 }
    return true
  })

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">Outillages & jauges</h1>
          <p className="text-sm text-muted-foreground">Suivi des étalonnages et cycles</p>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      <Tabs value={filter} onValueChange={(v) => setFilter(v as FilterMode)}>
        <TabsList>
          <TabsTrigger value="all">Tous ({tools.length})</TabsTrigger>
          <TabsTrigger value="expired">
            Expirés
            {expiredCount > 0 && <Badge variant="destructive" className="ml-1.5 px-1.5 py-0 text-xs">{expiredCount}</Badge>}
          </TabsTrigger>
          <TabsTrigger value="soon">
            Bientôt
            {soonCount > 0 && <Badge className="ml-1.5 px-1.5 py-0 text-xs bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0">{soonCount}</Badge>}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-3 text-destructive text-sm flex items-center gap-2">
            <AlertCircle className="size-4 shrink-0" />{error}
          </CardContent>
        </Card>
      )}

      {loading ? (
        <Skeleton className="h-48 w-full" />
      ) : displayed.length === 0 ? (
        <Card><CardContent className="p-8 text-center text-muted-foreground">Aucun outillage dans cette vue.</CardContent></Card>
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Nom / N° série</TableHead>
                <TableHead>Catégorie</TableHead>
                <TableHead>Prochain étalonnage</TableHead>
                <TableHead>Cycles</TableHead>
                <TableHead>Statut</TableHead>
                <TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {displayed.map((tool) => {
                const calDays = daysUntil(tool.next_calibration_at)
                return (
                  <TableRow key={tool.id}>
                    <TableCell>
                      <p className="font-medium">{tool.name ?? tool.serial_number ?? tool.id.slice(0, 8)}</p>
                      {tool.serial_number && <p className="text-xs text-muted-foreground font-mono">{tool.serial_number}</p>}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">{tool.category ?? '—'}</TableCell>
                    <TableCell className="text-sm">
                      {tool.next_calibration_at ? (
                        <span className={
                          calDays === null ? '' :
                          calDays < 0 ? 'text-red-600 dark:text-red-400 font-medium' :
                          calDays <= 30 ? 'text-yellow-600 dark:text-yellow-400 font-medium' : ''
                        }>
                          {new Date(tool.next_calibration_at).toLocaleDateString('fr-FR')}
                          {calDays !== null && calDays >= 0 && ` (J-${calDays})`}
                          {calDays !== null && calDays < 0 && ` (${Math.abs(calDays)}j retard)`}
                        </span>
                      ) : '—'}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {tool.max_cycles != null && tool.max_cycles > 0 ? (
                        <span className={(tool.current_cycles ?? 0) >= tool.max_cycles * 0.9 ? 'text-red-600 dark:text-red-400 font-medium' : ''}>
                          {tool.current_cycles ?? 0}/{tool.max_cycles}
                        </span>
                      ) : '—'}
                    </TableCell>
                    <TableCell>
                      <Badge className={statusClass(tool.status)}>{STATUS_LABEL[tool.status]}</Badge>
                    </TableCell>
                    <TableCell>
                      {tool.status !== 'TOOL_STATUS_DECOMMISSIONED' && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => { setCalibrateId(tool.id); setLastCal(new Date().toISOString().slice(0, 10)); setNextCal('') }}
                          disabled={busy === tool.id}
                        >
                          <Wrench className="size-3.5" />
                          Étalonner
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </Card>
      )}

      <Dialog open={!!calibrateId} onOpenChange={(open) => { if (!open) setCalibrateId(null) }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader><DialogTitle>Enregistrer un étalonnage</DialogTitle></DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="last-cal">Date d'étalonnage</Label>
              <Input id="last-cal" type="date" value={lastCal} onChange={(e) => setLastCal(e.target.value)} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="next-cal">Prochain étalonnage</Label>
              <Input id="next-cal" type="date" value={nextCal} onChange={(e) => setNextCal(e.target.value)} min={new Date().toISOString().slice(0, 10)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setCalibrateId(null)}>Annuler</Button>
            <Button onClick={handleCalibrate} disabled={!lastCal || !nextCal || busy !== null}>Confirmer</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
