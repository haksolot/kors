import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, Search, AlertCircle } from 'lucide-react'
import {
  listExpiringQualifications, listQualificationsByOperator,
  renewQualification, revokeQualification,
} from '@/lib/api'
import type { Qualification, QualificationStatus } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const STATUS_LABEL: Record<QualificationStatus, string> = {
  QUALIFICATION_STATUS_UNSPECIFIED: '—',
  QUALIFICATION_STATUS_ACTIVE: 'Active',
  QUALIFICATION_STATUS_EXPIRING: 'Expire bientôt',
  QUALIFICATION_STATUS_EXPIRED: 'Expirée',
  QUALIFICATION_STATUS_REVOKED: 'Révoquée',
}

function statusClass(s: QualificationStatus): string {
  switch (s) {
    case 'QUALIFICATION_STATUS_ACTIVE':   return 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
    case 'QUALIFICATION_STATUS_EXPIRING': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0'
    case 'QUALIFICATION_STATUS_EXPIRED':  return 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0'
    default:                              return 'bg-secondary text-secondary-foreground border-0'
  }
}

export default function QualificationsPage() {
  const [qualifications, setQualifications] = useState<Qualification[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [mode, setMode] = useState<'expiring' | 'search'>('expiring')
  const [searchId, setSearchId] = useState('')
  const [renewId, setRenewId] = useState<string | null>(null)
  const [renewDate, setRenewDate] = useState('')
  const [revokeId, setRevokeId] = useState<string | null>(null)
  const [revokeReason, setRevokeReason] = useState('')

  const loadExpiring = useCallback(async () => {
    setLoading(true)
    try {
      const res = await listExpiringQualifications(60)
      setQualifications(res.qualifications ?? [])
      setError(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally { setLoading(false) }
  }, [])

  useEffect(() => { loadExpiring() }, [loadExpiring])

  async function handleSearch() {
    if (!searchId.trim()) return
    setLoading(true)
    setError(null)
    try {
      const res = await listQualificationsByOperator(searchId.trim())
      setQualifications(res.qualifications ?? [])
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally { setLoading(false) }
  }

  async function handleRenew() {
    if (!renewId || !renewDate || busy) return
    setBusy(renewId)
    try {
      const res = await renewQualification(renewId, `${renewDate}T00:00:00Z`)
      setQualifications((prev) => prev.map((q) => (q.id === renewId ? res.qualification : q)))
      setRenewId(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur lors du renouvellement')
    } finally { setBusy(null) }
  }

  async function handleRevoke() {
    if (!revokeId || !revokeReason.trim() || busy) return
    setBusy(revokeId)
    try {
      const res = await revokeQualification(revokeId, revokeReason.trim())
      setQualifications((prev) => prev.map((q) => (q.id === revokeId ? res.qualification : q)))
      setRevokeId(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur lors de la révocation')
    } finally { setBusy(null) }
  }

  function refresh() {
    if (mode === 'expiring') loadExpiring()
    else handleSearch()
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-xl font-bold">Habilitations</h1>
          <p className="text-sm text-muted-foreground">Gestion des qualifications opérateurs (AS9100D §7.2)</p>
        </div>
        <Button variant="outline" size="sm" onClick={refresh} disabled={loading}>
          <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center">
        <Tabs value={mode} onValueChange={(v) => { setMode(v as 'expiring' | 'search'); if (v === 'expiring') loadExpiring() }}>
          <TabsList>
            <TabsTrigger value="expiring">Expirent bientôt</TabsTrigger>
            <TabsTrigger value="search">Par opérateur</TabsTrigger>
          </TabsList>
        </Tabs>

        {mode === 'search' && (
          <div className="flex gap-2 flex-1">
            <Input
              value={searchId}
              onChange={(e) => setSearchId(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
              placeholder="UUID opérateur…"
              className="font-mono text-sm flex-1"
            />
            <Button onClick={handleSearch} disabled={!searchId.trim()} size="sm">
              <Search className="size-4" />
              Rechercher
            </Button>
          </div>
        )}
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
      ) : qualifications.length === 0 ? (
        <Card>
          <CardContent className="p-8 text-center text-muted-foreground">
            {mode === 'expiring'
              ? "Aucune habilitation n'expire dans les 60 prochains jours."
              : 'Aucun résultat.'}
          </CardContent>
        </Card>
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Compétence</TableHead>
                <TableHead>Opérateur</TableHead>
                <TableHead>Expiration</TableHead>
                <TableHead>Statut</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {qualifications.map((q) => (
                <TableRow key={q.id}>
                  <TableCell className="font-medium">{q.label ?? q.skill}</TableCell>
                  <TableCell className="text-muted-foreground font-mono text-xs">
                    {q.operator_id.slice(0, 8)}…
                  </TableCell>
                  <TableCell className="text-sm">
                    {q.expires_at
                      ? <span className={
                          q.status === 'QUALIFICATION_STATUS_EXPIRED' ? 'text-red-600 dark:text-red-400 font-medium'
                          : q.status === 'QUALIFICATION_STATUS_EXPIRING' ? 'text-yellow-600 dark:text-yellow-400 font-medium'
                          : ''
                        }>
                          {new Date(q.expires_at).toLocaleDateString('fr-FR')}
                        </span>
                      : '—'
                    }
                  </TableCell>
                  <TableCell>
                    <Badge className={statusClass(q.status)}>{STATUS_LABEL[q.status]}</Badge>
                  </TableCell>
                  <TableCell>
                    {!q.is_revoked && q.status !== 'QUALIFICATION_STATUS_REVOKED' && (
                      <div className="flex gap-2">
                        <Button variant="outline" size="sm" onClick={() => { setRenewId(q.id); setRenewDate('') }}>
                          Renouveler
                        </Button>
                        <Button variant="outline" size="sm" className="text-destructive border-destructive/30 hover:bg-destructive/10" onClick={() => { setRevokeId(q.id); setRevokeReason('') }}>
                          Révoquer
                        </Button>
                      </div>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      {/* Renew dialog */}
      <Dialog open={!!renewId} onOpenChange={(open) => { if (!open) setRenewId(null) }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader><DialogTitle>Renouveler l'habilitation</DialogTitle></DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="renew-date">Nouvelle date d'expiration</Label>
            <Input id="renew-date" type="date" value={renewDate} onChange={(e) => setRenewDate(e.target.value)} min={new Date().toISOString().slice(0, 10)} />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setRenewId(null)}>Annuler</Button>
            <Button onClick={handleRenew} disabled={!renewDate || busy !== null}>Confirmer</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Revoke dialog */}
      <Dialog open={!!revokeId} onOpenChange={(open) => { if (!open) setRevokeId(null) }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader><DialogTitle>Révoquer l'habilitation</DialogTitle></DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="revoke-reason">Motif (obligatoire)</Label>
            <Textarea id="revoke-reason" value={revokeReason} onChange={(e) => setRevokeReason(e.target.value)} placeholder="Raison de la révocation…" rows={3} />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setRevokeId(null)}>Annuler</Button>
            <Button variant="destructive" onClick={handleRevoke} disabled={!revokeReason.trim() || busy !== null}>
              Révoquer
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
