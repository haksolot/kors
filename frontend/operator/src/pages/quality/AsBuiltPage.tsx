import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, ChevronDown, ChevronRight, CheckCircle2, AlertCircle, Wrench, Boxes, FlaskConical } from 'lucide-react'
import { getAsBuilt } from '@/lib/api'
import type { AsBuiltReport } from '@/lib/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

function formatDuration(s?: number): string {
  if (!s) return '—'
  return `${Math.floor(s / 60)} min`
}

function OperationBlock({ op }: { op: AsBuiltReport['operations'][number] }) {
  const [open, setOpen] = useState(false)
  const hasDetails = op.measurements?.length > 0 || op.consumed_lots?.length > 0 || op.tools?.length > 0
  const isDone = op.status === 'OPERATION_STATUS_COMPLETED' || op.status === 'OPERATION_STATUS_RELEASED'

  return (
    <Card className="overflow-hidden">
      <button
        onClick={() => hasDetails && setOpen((v) => !v)}
        className={`w-full text-left px-4 py-3 flex items-center gap-3 ${hasDetails ? 'hover:bg-accent cursor-pointer' : 'cursor-default'} transition-colors`}
      >
        <div className={`size-7 rounded-full flex items-center justify-center text-xs font-bold shrink-0 ${
          isDone ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300'
          : op.status === 'OPERATION_STATUS_IN_PROGRESS' ? 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300'
          : 'bg-muted text-muted-foreground'
        }`}>
          {op.step_number}
        </div>
        <div className="flex-1 min-w-0">
          <p className="font-medium truncate">{op.name}</p>
          <div className="flex flex-wrap gap-3 text-xs text-muted-foreground mt-0.5">
            {op.actual_duration_seconds != null && (
              <span>Durée : {formatDuration(op.actual_duration_seconds)} / {formatDuration(op.planned_duration_seconds)}</span>
            )}
            {op.is_special_process && (
              <Badge className="bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300 border-0 text-xs">
                NADCAP {op.nadcap_process_code}
              </Badge>
            )}
            {op.requires_sign_off && (
              op.signed_off_by
                ? <span className="text-green-600 dark:text-green-400 flex items-center gap-0.5"><CheckCircle2 className="size-3" />Visé</span>
                : <span className="text-yellow-600 dark:text-yellow-400">Visa requis</span>
            )}
          </div>
        </div>
        {hasDetails && (open ? <ChevronDown className="size-4 text-muted-foreground shrink-0" /> : <ChevronRight className="size-4 text-muted-foreground shrink-0" />)}
      </button>

      {open && hasDetails && (
        <div className="border-t bg-muted/30 px-4 pb-4 pt-3 flex flex-col gap-4">
          {op.measurements?.length > 0 && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2 flex items-center gap-1.5">
                <FlaskConical className="size-3.5" />Mesures qualité
              </p>
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                {op.measurements.map((m, i) => (
                  <div key={i} className={`rounded-lg p-2 text-sm border ${m.status === 'PASS' ? 'bg-green-50 border-green-200 dark:bg-green-950/30 dark:border-green-800' : 'bg-red-50 border-red-200 dark:bg-red-950/30 dark:border-red-800'}`}>
                    <p className="text-xs text-muted-foreground truncate">{m.characteristic_id.slice(0, 8)}…</p>
                    <p className="font-bold">{m.value}</p>
                    <p className={`text-xs font-medium ${m.status === 'PASS' ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>{m.status}</p>
                  </div>
                ))}
              </div>
            </div>
          )}
          {op.consumed_lots?.length > 0 && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2 flex items-center gap-1.5">
                <Boxes className="size-3.5" />Matières consommées
              </p>
              <div className="flex flex-col gap-1">
                {op.consumed_lots.map((lot, i) => (
                  <div key={i} className="flex justify-between text-sm">
                    <span className="font-mono text-xs text-muted-foreground">{lot.lot_id}</span>
                    <span className="text-muted-foreground">Qté {lot.quantity}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
          {op.tools?.length > 0 && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2 flex items-center gap-1.5">
                <Wrench className="size-3.5" />Outillages
              </p>
              <div className="flex flex-col gap-1">
                {op.tools.map((t, i) => (
                  <div key={i} className="flex justify-between text-sm">
                    <span>{t.name ?? t.serial_number ?? t.tool_id.slice(0, 8)}</span>
                    {t.calibration_expiry && (
                      <span className={`text-xs ${new Date(t.calibration_expiry) < new Date() ? 'text-red-600 dark:text-red-400' : 'text-muted-foreground'}`}>
                        Cal. {new Date(t.calibration_expiry).toLocaleDateString('fr-FR')}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </Card>
  )
}

export default function AsBuiltPage() {
  const { ofId } = useParams<{ ofId: string }>()
  const navigate = useNavigate()
  const [searchId, setSearchId] = useState(ofId ?? '')
  const [report, setReport] = useState<AsBuiltReport | null>(null)
  const [loading, setLoading] = useState(!!ofId)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!ofId) return
    setLoading(true)
    getAsBuilt(ofId)
      .then(setReport)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Erreur'))
      .finally(() => setLoading(false))
  }, [ofId])

  async function handleSearch() {
    if (!searchId.trim()) return
    navigate(`/quality/as-built/${searchId.trim()}`)
  }

  const ops = [...(report?.operations ?? [])].sort((a, b) => a.step_number - b.step_number)

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-bold">Dossier As-Built</h1>
        <p className="text-sm text-muted-foreground">Dossier de fabrication numérique — EN9100 §8.5.2</p>
      </div>

      {/* Search by OF ID */}
      {!ofId && (
        <Card>
          <CardContent className="p-4 flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="of-id">Référence OF (UUID)</Label>
              <div className="flex gap-2">
                <Input id="of-id" value={searchId} onChange={(e) => setSearchId(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && handleSearch()} placeholder="UUID de l'OF…" className="font-mono text-sm flex-1" />
                <Button onClick={handleSearch} disabled={!searchId.trim()}>Charger</Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {ofId && (
        <Button variant="ghost" size="sm" className="self-start" onClick={() => navigate('/quality/as-built')}>
          <ArrowLeft className="size-4" />
          Autre OF
        </Button>
      )}

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-4 text-destructive text-sm flex items-center gap-2">
            <AlertCircle className="size-4" />{error}
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-24 w-full" />
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
        </div>
      ) : report && (
        <>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">OF : {report.reference ?? ofId}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-xs text-muted-foreground">Produit</p>
                  <p className="font-medium">{report.product_id?.slice(0, 8) ?? '—'}…</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Quantité</p>
                  <p className="font-medium">{report.quantity ?? '—'}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">N° série</p>
                  <p className="font-medium font-mono text-xs">
                    {Array.isArray(report.serial_numbers) && report.serial_numbers.length > 0
                      ? String(report.serial_numbers[0])
                      : '—'}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>

          <Separator />

          <div>
            <p className="text-sm font-medium text-muted-foreground mb-3">
              Opérations ({ops.length})
            </p>
            <div className="flex flex-col gap-2">
              {ops.map((op) => <OperationBlock key={op.operation_id} op={op} />)}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
