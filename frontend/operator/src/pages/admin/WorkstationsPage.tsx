import { useCallback, useEffect, useState } from 'react'
import { Plus, RefreshCw, AlertCircle } from 'lucide-react'
import { listWorkstations, createWorkstation, updateWorkstationStatus } from '@/lib/api'
import type { Workstation, WorkstationStatus } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'

const STATUS_LABEL: Record<WorkstationStatus, string> = {
  WORKSTATION_STATUS_UNSPECIFIED: '—',
  WORKSTATION_STATUS_AVAILABLE: 'Disponible',
  WORKSTATION_STATUS_IN_PRODUCTION: 'En production',
  WORKSTATION_STATUS_DOWN: 'En panne',
  WORKSTATION_STATUS_MAINTENANCE: 'Maintenance',
}

function statusClass(s: WorkstationStatus): string {
  switch (s) {
    case 'WORKSTATION_STATUS_AVAILABLE':     return 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
    case 'WORKSTATION_STATUS_IN_PRODUCTION': return 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0'
    case 'WORKSTATION_STATUS_DOWN':          return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    case 'WORKSTATION_STATUS_MAINTENANCE':   return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    default:                                 return 'bg-secondary text-secondary-foreground border-0'
  }
}

const SETTABLE: WorkstationStatus[] = [
  'WORKSTATION_STATUS_AVAILABLE',
  'WORKSTATION_STATUS_DOWN',
  'WORKSTATION_STATUS_MAINTENANCE',
]

export default function WorkstationsPage() {
  const [workstations, setWorkstations] = useState<Workstation[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [newCap, setNewCap] = useState('1')
  const [newRate, setNewRate] = useState('0')

  const [statusId, setStatusId] = useState<string | null>(null)
  const [selectedStatus, setSelectedStatus] = useState<WorkstationStatus>('WORKSTATION_STATUS_AVAILABLE')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await listWorkstations()
      setWorkstations(res.workstations ?? [])
      setError(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally { setLoading(false) }
  }, [])

  useEffect(() => { load() }, [load])

  async function handleCreate() {
    if (!newName.trim() || busy) return
    setBusy('create')
    try {
      const res = await createWorkstation(newName.trim(), newDesc.trim(), parseInt(newCap) || 1, parseFloat(newRate) || 0)
      setWorkstations((prev) => [res.workstation, ...prev])
      setShowCreate(false)
      setNewName(''); setNewDesc(''); setNewCap('1'); setNewRate('0')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur création')
    } finally { setBusy(null) }
  }

  async function handleStatusChange() {
    if (!statusId || busy) return
    setBusy(statusId)
    try {
      const res = await updateWorkstationStatus(statusId, selectedStatus)
      setWorkstations((prev) => prev.map((w) => (w.id === statusId ? res.workstation : w)))
      setStatusId(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur changement de statut')
    } finally { setBusy(null) }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">Postes de travail</h1>
          <p className="text-sm text-muted-foreground">{workstations.length} poste(s) configuré(s)</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={load} disabled={loading}>
            <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
          </Button>
          <Button size="sm" onClick={() => setShowCreate((v) => !v)}>
            <Plus className="size-4" />
            Nouveau poste
          </Button>
        </div>
      </div>

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-3 text-destructive text-sm flex items-center gap-2">
            <AlertCircle className="size-4 shrink-0" />{error}
          </CardContent>
        </Card>
      )}

      {/* Create form inline */}
      {showCreate && (
        <Card className="border-primary/30">
          <CardContent className="p-4 flex flex-col gap-4">
            <h3 className="font-semibold text-sm">Nouveau poste de travail</h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ws-name">Nom *</Label>
                <Input id="ws-name" value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="ex: Tour CN-01" />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ws-desc">Description</Label>
                <Input id="ws-desc" value={newDesc} onChange={(e) => setNewDesc(e.target.value)} placeholder="Description…" />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ws-cap">Capacité (pièces)</Label>
                <Input id="ws-cap" type="number" min="1" value={newCap} onChange={(e) => setNewCap(e.target.value)} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ws-rate">Cadence nominale (p/h)</Label>
                <Input id="ws-rate" type="number" min="0" step="0.1" value={newRate} onChange={(e) => setNewRate(e.target.value)} />
              </div>
            </div>
            <div className="flex gap-2">
              <Button onClick={handleCreate} disabled={!newName.trim() || busy !== null}>
                Créer le poste
              </Button>
              <Button variant="ghost" onClick={() => setShowCreate(false)}>Annuler</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-32" />)}
        </div>
      ) : workstations.length === 0 ? (
        <Card><CardContent className="p-8 text-center text-muted-foreground">Aucun poste configuré.</CardContent></Card>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {workstations.map((ws) => (
            <Card key={ws.id}>
              <CardContent className="p-4">
                <div className="flex items-start justify-between gap-2 mb-2">
                  <p className="font-semibold truncate flex-1">{ws.name}</p>
                  <Badge className={statusClass(ws.status)}>{STATUS_LABEL[ws.status]}</Badge>
                </div>
                {ws.description && <p className="text-sm text-muted-foreground mb-2">{ws.description}</p>}
                <div className="flex gap-3 text-xs text-muted-foreground mb-3">
                  {ws.capacity != null && <span>Cap. {ws.capacity} pcs</span>}
                  {ws.nominal_rate != null && ws.nominal_rate > 0 && <span>Cadence {ws.nominal_rate} p/h</span>}
                </div>
                <Separator className="mb-3" />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => { setStatusId(ws.id); setSelectedStatus('WORKSTATION_STATUS_AVAILABLE') }}
                  disabled={busy === ws.id}
                >
                  Changer le statut
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={!!statusId} onOpenChange={(open) => { if (!open) setStatusId(null) }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader><DialogTitle>Changer le statut du poste</DialogTitle></DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label>Nouveau statut</Label>
            <Select value={selectedStatus} onValueChange={(v) => setSelectedStatus(v as WorkstationStatus)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {SETTABLE.map((s) => (
                  <SelectItem key={s} value={s}>{STATUS_LABEL[s]}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setStatusId(null)}>Annuler</Button>
            <Button onClick={handleStatusChange} disabled={busy !== null}>Confirmer</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
