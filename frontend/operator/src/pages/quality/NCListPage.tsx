import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, AlertCircle } from 'lucide-react'
import { listNCs, startAnalysis, proposeDisposition, closeNC } from '@/lib/api'
import type { NonConformity, NCStatus, NCDisposition } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const STATUS_LABEL: Record<NCStatus, string> = {
  NC_STATUS_UNSPECIFIED: '—',
  NC_STATUS_OPEN: 'Ouverte',
  NC_STATUS_UNDER_ANALYSIS: 'En analyse',
  NC_STATUS_PENDING_DISPOSITION: 'Attente décision',
  NC_STATUS_CLOSED: 'Clôturée',
}

function statusClass(s: NCStatus): string {
  switch (s) {
    case 'NC_STATUS_OPEN':                return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    case 'NC_STATUS_UNDER_ANALYSIS':      return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    case 'NC_STATUS_PENDING_DISPOSITION': return 'bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300 border-0'
    case 'NC_STATUS_CLOSED':              return 'bg-secondary text-secondary-foreground border-0'
    default:                              return 'bg-secondary text-secondary-foreground border-0'
  }
}

const DISPOSITION_LABEL: Record<NCDisposition, string> = {
  NC_DISPOSITION_UNSPECIFIED: '— Sélectionner —',
  NC_DISPOSITION_REWORK: 'Retouche',
  NC_DISPOSITION_SCRAP: 'Rebut',
  NC_DISPOSITION_USE_AS_IS: 'Dérogation',
  NC_DISPOSITION_RETURN_TO_SUPPLIER: 'Retour fournisseur',
}

const DISPOSITIONS: NCDisposition[] = [
  'NC_DISPOSITION_REWORK',
  'NC_DISPOSITION_SCRAP',
  'NC_DISPOSITION_USE_AS_IS',
  'NC_DISPOSITION_RETURN_TO_SUPPLIER',
]

export default function NCListPage() {
  const [ncs, setNCs] = useState<NonConformity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [dispositionId, setDispositionId] = useState<string | null>(null)
  const [selectedDisp, setSelectedDisp] = useState<NCDisposition>('NC_DISPOSITION_UNSPECIFIED')

  const load = useCallback(async () => {
    try {
      const res = await listNCs()
      setNCs(res.ncs ?? [])
      setError(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  async function action(id: string, fn: () => Promise<{ nc: NonConformity }>) {
    setBusy(id)
    try {
      const res = await fn()
      setNCs((prev) => prev.map((nc) => (nc.id === id ? res.nc : nc)))
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally {
      setBusy(null)
    }
  }

  async function handleDisposition() {
    if (!dispositionId || selectedDisp === 'NC_DISPOSITION_UNSPECIFIED') return
    const id = dispositionId
    setDispositionId(null)
    await action(id, () => proposeDisposition(id, selectedDisp))
  }

  const open   = ncs.filter((nc) => nc.status !== 'NC_STATUS_CLOSED')
  const closed = ncs.filter((nc) => nc.status === 'NC_STATUS_CLOSED')

  const NCRow = ({ nc }: { nc: NonConformity }) => (
    <TableRow key={nc.id}>
      <TableCell className="font-medium">
        {nc.defect_code && <span className="text-xs text-muted-foreground font-mono mr-1">[{nc.defect_code}]</span>}
        {nc.description ?? 'Sans description'}
      </TableCell>
      <TableCell className="text-muted-foreground text-xs">
        {nc.created_at ? new Date(nc.created_at).toLocaleDateString('fr-FR') : '—'}
      </TableCell>
      <TableCell className="text-right text-muted-foreground">{nc.affected_quantity ?? '—'}</TableCell>
      <TableCell>
        <Badge className={statusClass(nc.status)}>{STATUS_LABEL[nc.status]}</Badge>
      </TableCell>
      <TableCell>
        <div className="flex gap-2">
          {nc.status === 'NC_STATUS_OPEN' && (
            <Button variant="outline" size="sm" onClick={() => action(nc.id, () => startAnalysis(nc.id))} disabled={busy === nc.id}>
              Analyser
            </Button>
          )}
          {nc.status === 'NC_STATUS_UNDER_ANALYSIS' && (
            <Button variant="outline" size="sm" onClick={() => { setDispositionId(nc.id); setSelectedDisp('NC_DISPOSITION_UNSPECIFIED') }} disabled={busy === nc.id}>
              Décision
            </Button>
          )}
          {nc.status === 'NC_STATUS_PENDING_DISPOSITION' && (
            <Button variant="outline" size="sm" className="text-green-700 dark:text-green-400 border-green-300 dark:border-green-700" onClick={() => action(nc.id, () => closeNC(nc.id))} disabled={busy === nc.id}>
              Clôturer
            </Button>
          )}
        </div>
      </TableCell>
    </TableRow>
  )

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">
            Non-conformités
            {open.length > 0 && <span className="ml-2 text-red-600 dark:text-red-400">({open.length} ouvertes)</span>}
          </h1>
          <p className="text-sm text-muted-foreground">Gestion des NC — workflow qualité</p>
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
      ) : ncs.length === 0 ? (
        <Card><CardContent className="p-8 text-center text-muted-foreground">Aucune non-conformité enregistrée.</CardContent></Card>
      ) : (
        <Tabs defaultValue="open">
          <TabsList>
            <TabsTrigger value="open">
              Ouvertes
              {open.length > 0 && <Badge variant="destructive" className="ml-1.5 px-1.5 py-0 text-xs">{open.length}</Badge>}
            </TabsTrigger>
            <TabsTrigger value="closed">Clôturées ({closed.length})</TabsTrigger>
          </TabsList>

          {(['open', 'closed'] as const).map((tab) => (
            <TabsContent key={tab} value={tab}>
              <Card>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Description</TableHead>
                      <TableHead>Date</TableHead>
                      <TableHead className="text-right">Qté</TableHead>
                      <TableHead>Statut</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(tab === 'open' ? open : closed).map((nc) => <NCRow key={nc.id} nc={nc} />)}
                  </TableBody>
                </Table>
              </Card>
            </TabsContent>
          ))}
        </Tabs>
      )}

      <Dialog open={!!dispositionId} onOpenChange={(open) => { if (!open) setDispositionId(null) }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader><DialogTitle>Proposer une décision</DialogTitle></DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label>Disposition</Label>
            <Select value={selectedDisp} onValueChange={(v) => setSelectedDisp(v as NCDisposition)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {DISPOSITIONS.map((d) => (
                  <SelectItem key={d} value={d}>{DISPOSITION_LABEL[d]}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDispositionId(null)}>Annuler</Button>
            <Button onClick={handleDisposition} disabled={selectedDisp === 'NC_DISPOSITION_UNSPECIFIED'}>
              Confirmer
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
