import { useState } from 'react'
import { Search } from 'lucide-react'
import { queryAuditTrail } from '@/lib/api'
import type { AuditEntry } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function isoDateMinus(days: number) {
  const d = new Date()
  d.setDate(d.getDate() - days)
  return d.toISOString().slice(0, 10)
}

export default function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [searched, setSearched] = useState(false)

  const [actorId, setActorId] = useState('')
  const [entityType, setEntityType] = useState('')
  const [entityId, setEntityId] = useState('')
  const [action, setAction] = useState('')
  const [from, setFrom] = useState(isoDateMinus(7))
  const [to, setTo] = useState(new Date().toISOString().slice(0, 10))

  async function search() {
    setLoading(true)
    setError(null)
    try {
      const res = await queryAuditTrail({
        actorId: actorId.trim() || undefined,
        entityType: entityType.trim() || undefined,
        entityId: entityId.trim() || undefined,
        action: action.trim() || undefined,
        from: from ? `${from}T00:00:00Z` : undefined,
        to: to ? `${to}T23:59:59Z` : undefined,
      })
      setEntries(res.entries ?? [])
      setSearched(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-xl font-bold">Audit trail</h1>
        <p className="text-sm text-muted-foreground">Traçabilité complète des actions — EN9100 §8.5.2</p>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="a-actor">Acteur (UUID)</Label>
              <Input id="a-actor" value={actorId} onChange={(e) => setActorId(e.target.value)} placeholder="UUID…" className="font-mono text-sm" />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="a-type">Type d'entité</Label>
              <Input id="a-type" value={entityType} onChange={(e) => setEntityType(e.target.value)} placeholder="ex: order, operation…" className="text-sm" />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="a-entity">ID entité (UUID)</Label>
              <Input id="a-entity" value={entityId} onChange={(e) => setEntityId(e.target.value)} placeholder="UUID…" className="font-mono text-sm" />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="a-action">Action</Label>
              <Input id="a-action" value={action} onChange={(e) => setAction(e.target.value)} placeholder="ex: create, update…" className="text-sm" />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="a-from">Du</Label>
              <Input id="a-from" type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="a-to">Au</Label>
              <Input id="a-to" type="date" value={to} onChange={(e) => setTo(e.target.value)} />
            </div>
          </div>
          <Separator className="my-3" />
          <Button onClick={search} disabled={loading}>
            <Search className="size-4" />
            Rechercher
          </Button>
        </CardContent>
      </Card>

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-3 text-destructive text-sm">{error}</CardContent>
        </Card>
      )}

      {loading ? (
        <Skeleton className="h-48 w-full" />
      ) : searched ? (
        entries.length === 0 ? (
          <Card><CardContent className="p-8 text-center text-muted-foreground">Aucune entrée pour ces critères.</CardContent></Card>
        ) : (
          <Card>
            <div className="text-xs text-muted-foreground px-4 py-2 border-b">
              {entries.length} entrée(s) trouvée(s)
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Horodatage</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Entité</TableHead>
                  <TableHead>Acteur</TableHead>
                  <TableHead>Détails</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map((e) => (
                  <TableRow key={e.id}>
                    <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                      {e.created_at ? new Date(e.created_at).toLocaleString('fr-FR') : '—'}
                    </TableCell>
                    <TableCell>
                      <span className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded">{e.action}</span>
                    </TableCell>
                    <TableCell className="text-xs">
                      <span className="text-muted-foreground">{e.entity_type}</span>
                      {e.entity_id && (
                        <span className="ml-1 font-mono text-foreground/60">{e.entity_id.slice(0, 8)}…</span>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {e.actor_id ? `${e.actor_id.slice(0, 8)}…` : '—'}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground max-w-xs truncate">
                      {e.notes ?? '—'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>
        )
      ) : (
        <Card><CardContent className="p-8 text-center text-muted-foreground">Lancez une recherche pour afficher les entrées d'audit.</CardContent></Card>
      )}
    </div>
  )
}
