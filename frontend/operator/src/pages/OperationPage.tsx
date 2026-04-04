import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  ArrowLeft, Play, Pause, CheckCircle2, TriangleAlert, PackageSearch,
  PauseOctagon, Timer, Clipboard, FlaskConical, Boxes,
} from 'lucide-react'
import {
  fetchOperation, fetchCharacteristics, startOperation, completeOperation,
  recordMeasurement, consumeMaterial, recordTimeLog, raiseAlert, suspendOrder,
} from '@/lib/api'
import type { AlertCategory, ControlCharacteristic, Operation } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'

// ── Timer hook ────────────────────────────────────────────────────────────────

function useTimer(running: boolean) {
  const [elapsed, setElapsed] = useState(0)
  const startRef = useRef<number | null>(null)
  const accRef = useRef(0)

  useEffect(() => {
    if (running) {
      startRef.current = Date.now()
      const id = setInterval(() => {
        setElapsed(accRef.current + Math.floor((Date.now() - startRef.current!) / 1000))
      }, 1000)
      return () => clearInterval(id)
    } else {
      if (startRef.current !== null) {
        accRef.current += Math.floor((Date.now() - startRef.current) / 1000)
        startRef.current = null
      }
    }
  }, [running])

  const reset = useCallback(() => { accRef.current = 0; startRef.current = null; setElapsed(0) }, [])
  const snapshot = running
    ? accRef.current + Math.floor((Date.now() - (startRef.current ?? Date.now())) / 1000)
    : accRef.current

  return { elapsed, snapshot, reset }
}

function fmt(s: number) {
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}

// ── Alert categories ──────────────────────────────────────────────────────────

const ALERT_OPTIONS: { category: AlertCategory; label: string; icon: React.ElementType; color: string }[] = [
  { category: 'ALERT_CATEGORY_MACHINE',   label: 'Panne machine',     icon: PauseOctagon, color: 'text-red-600 dark:text-red-400' },
  { category: 'ALERT_CATEGORY_LOGISTICS', label: 'Manque matière',    icon: Boxes,        color: 'text-yellow-600 dark:text-yellow-400' },
  { category: 'ALERT_CATEGORY_QUALITY',   label: 'Problème qualité',  icon: TriangleAlert,color: 'text-orange-600 dark:text-orange-400' },
  { category: 'ALERT_CATEGORY_PLANNING',  label: 'Autre / Planning',  icon: Clipboard,    color: 'text-blue-600 dark:text-blue-400' },
]

// ── Page ──────────────────────────────────────────────────────────────────────

export default function OperationPage() {
  const { id: ofId, op_id: opId } = useParams<{ id: string; op_id: string }>()
  const navigate = useNavigate()

  const [operation, setOperation] = useState<Operation | null>(null)
  const [characteristics, setCharacteristics] = useState<ControlCharacteristic[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const [timerRunning, setTimerRunning] = useState(false)
  const { elapsed, snapshot, reset: resetTimer } = useTimer(timerRunning)

  const [measurements, setMeasurements] = useState<Record<string, string>>({})
  const [measureErrors, setMeasureErrors] = useState<Record<string, string>>({})

  const [lotId, setLotId] = useState('')
  const [qty, setQty] = useState('1')
  const [materialMsg, setMaterialMsg] = useState<{ ok: boolean; text: string } | null>(null)

  const [suspendReason, setSuspendReason] = useState('')
  const [showSuspend, setShowSuspend] = useState(false)

  const [alertOpen, setAlertOpen] = useState(false)
  const [alertCategory, setAlertCategory] = useState<typeof ALERT_OPTIONS[number] | null>(null)
  const [alertConfirm, setAlertConfirm] = useState(false)

  useEffect(() => {
    if (!ofId || !opId) return
    setLoading(true)
    Promise.all([fetchOperation(ofId, opId), fetchCharacteristics(opId)])
      .then(([opRes, charRes]) => {
        setOperation(opRes.operation)
        setCharacteristics(charRes.characteristics ?? [])
        if (opRes.operation.status === 'OPERATION_STATUS_IN_PROGRESS') setTimerRunning(true)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Erreur'))
      .finally(() => setLoading(false))
  }, [ofId, opId])

  async function handleStart() {
    if (!ofId || !opId || busy) return
    setBusy(true)
    try {
      const res = await startOperation(ofId, opId)
      setOperation(res.operation)
      setTimerRunning(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur au démarrage')
    } finally { setBusy(false) }
  }

  async function handleComplete() {
    if (!ofId || !opId || busy) return
    const missing = characteristics
      .filter((c) => c.is_mandatory && !measurements[c.id]?.trim())
      .map((c) => c.id)
    if (missing.length > 0) {
      const errs: Record<string, string> = {}
      missing.forEach((id) => { errs[id] = 'Valeur obligatoire' })
      setMeasureErrors(errs)
      return
    }
    setBusy(true)
    try {
      await Promise.all(
        Object.entries(measurements)
          .filter(([, v]) => v.trim())
          .map(([charId, value]) => recordMeasurement(opId!, charId, value)),
      )
      if (snapshot > 0) {
        await recordTimeLog(ofId, opId, snapshot).catch(() => {})
      }
      const res = await completeOperation(ofId, opId)
      setOperation(res.operation)
      setTimerRunning(false)
      resetTimer()
      navigate(`/order/${ofId}`)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur à la complétion')
    } finally { setBusy(false) }
  }

  async function handleConsumeMaterial() {
    if (!opId || !lotId.trim() || busy) return
    setBusy(true)
    try {
      await consumeMaterial(opId, lotId.trim(), parseInt(qty, 10) || 1)
      setMaterialMsg({ ok: true, text: 'Consommation enregistrée.' })
      setLotId('')
      setQty('1')
    } catch (err: unknown) {
      setMaterialMsg({ ok: false, text: err instanceof Error ? err.message : 'Erreur' })
    } finally { setBusy(false) }
  }

  async function handleRaiseAlert() {
    if (!opId || !alertCategory || busy) return
    setBusy(true)
    try {
      await raiseAlert(alertCategory.category, opId, alertCategory.label)
      setAlertOpen(false)
      setAlertConfirm(false)
      setAlertCategory(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur lors du signalement')
    } finally { setBusy(false) }
  }

  async function handleSuspend() {
    if (!ofId || !suspendReason || busy) return
    setBusy(true)
    try {
      await suspendOrder(ofId, suspendReason)
      navigate(`/order/${ofId}`)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur lors de la suspension')
    } finally { setBusy(false) }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-4 max-w-2xl mx-auto">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-60 w-full" />
      </div>
    )
  }

  if (error || !operation) {
    return (
      <div className="max-w-2xl mx-auto flex flex-col gap-4">
        <Alert variant="destructive">
          <AlertDescription>{error ?? 'Opération introuvable'}</AlertDescription>
        </Alert>
        <Button variant="outline" onClick={() => navigate(`/order/${ofId}`)}>
          <ArrowLeft className="size-4" />
          Retour à l'OF
        </Button>
      </div>
    )
  }

  const isPending    = operation.status === 'OPERATION_STATUS_PENDING'
  const isInProgress = operation.status === 'OPERATION_STATUS_IN_PROGRESS'
  const isDone       = ['OPERATION_STATUS_COMPLETED', 'OPERATION_STATUS_RELEASED', 'OPERATION_STATUS_PENDING_SIGN_OFF'].includes(operation.status)

  return (
    <div className="flex flex-col gap-4 max-w-2xl mx-auto pb-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate(`/order/${ofId}`)}>
          <ArrowLeft className="size-5" />
        </Button>
        <div className="flex-1 min-w-0">
          <p className="text-xs text-muted-foreground">Étape {operation.step_number}</p>
          <h1 className="text-lg font-bold truncate">{operation.name}</h1>
        </div>
        <Button
          variant="destructive"
          size="sm"
          onClick={() => { setAlertOpen(true); setAlertConfirm(false); setAlertCategory(null) }}
        >
          <TriangleAlert className="size-4" />
          Aléa
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>
            {error}
            <button className="ml-2 underline" onClick={() => setError(null)}>Fermer</button>
          </AlertDescription>
        </Alert>
      )}

      {/* Timer + primary actions */}
      <Card>
        <CardContent className="p-5 flex flex-col items-center gap-4">
          <div className="flex flex-col items-center gap-1">
            <p className="font-mono text-6xl font-bold tracking-widest tabular-nums">
              {fmt(elapsed)}
            </p>
            {operation.planned_duration_seconds != null && operation.planned_duration_seconds > 0 && (
              <p className="text-sm text-muted-foreground flex items-center gap-1">
                <Timer className="size-3.5" />
                Alloué : {fmt(operation.planned_duration_seconds)}
              </p>
            )}
          </div>

          <div className="flex gap-3 w-full">
            {isPending && (
              <Button size="lg" className="flex-1 h-14 text-base bg-green-600 hover:bg-green-700 dark:bg-green-700 dark:hover:bg-green-600 text-white" onClick={handleStart} disabled={busy}>
                <Play className="size-5" />
                Démarrer
              </Button>
            )}
            {isInProgress && (
              <>
                <Button size="lg" variant="outline" className="flex-1 h-14 text-base" onClick={() => setTimerRunning((v) => !v)}>
                  {timerRunning ? <><Pause className="size-5" />Pause</> : <><Play className="size-5" />Reprendre</>}
                </Button>
                <Button size="lg" className="flex-1 h-14 text-base" onClick={handleComplete} disabled={busy}>
                  <CheckCircle2 className="size-5" />
                  Terminer
                </Button>
              </>
            )}
            {isDone && (
              <div className="flex-1 flex items-center justify-center gap-2 text-green-600 dark:text-green-400 font-semibold">
                <CheckCircle2 className="size-5" />
                Opération terminée
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Tabs for secondary tasks */}
      <Tabs defaultValue="quality">
        <TabsList className="w-full">
          <TabsTrigger value="quality" className="flex-1">
            <FlaskConical className="size-4" />
            Contrôles
            {characteristics.filter((c) => c.is_mandatory && !measurements[c.id]).length > 0 && (
              <Badge variant="destructive" className="ml-1 text-xs px-1.5 py-0">
                {characteristics.filter((c) => c.is_mandatory && !measurements[c.id]).length}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="material" className="flex-1" disabled={!isInProgress}>
            <PackageSearch className="size-4" />
            Matière
          </TabsTrigger>
          {operation.instructions_url && (
            <TabsTrigger value="instructions" className="flex-1">
              <Clipboard className="size-4" />
              Instructions
            </TabsTrigger>
          )}
        </TabsList>

        {/* Quality measurements */}
        <TabsContent value="quality">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">Contrôles qualité</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              {characteristics.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-4">Aucun contrôle requis pour cette opération.</p>
              ) : (
                characteristics.map((c) => {
                  const val = measurements[c.id] ?? ''
                  const isQuant = c.type === 'CHARACTERISTIC_TYPE_QUANTITATIVE'
                  const numVal = parseFloat(val)
                  const inTol = isQuant && val.trim()
                    ? numVal >= (c.nominal_value ?? 0) - (c.lower_tolerance ?? 0) &&
                      numVal <= (c.nominal_value ?? 0) + (c.upper_tolerance ?? 0)
                    : null

                  return (
                    <div key={c.id} className="flex flex-col gap-1.5">
                      <div className="flex items-center justify-between">
                        <Label className="font-medium">
                          {c.name}
                          {c.is_mandatory && <span className="text-destructive ml-1">*</span>}
                        </Label>
                        {isQuant && c.nominal_value != null && (
                          <span className="text-xs text-muted-foreground">
                            {c.nominal_value} ± {c.upper_tolerance ?? 0} {c.unit}
                          </span>
                        )}
                      </div>
                      <div className="flex gap-2 items-center">
                        <Input
                          type={isQuant ? 'number' : 'text'}
                          step="any"
                          value={val}
                          onChange={(e) => {
                            setMeasurements((m) => ({ ...m, [c.id]: e.target.value }))
                            setMeasureErrors((er) => { const n = { ...er }; delete n[c.id]; return n })
                          }}
                          placeholder={isQuant ? `${c.nominal_value ?? '—'} ${c.unit ?? ''}` : 'OK / NOK'}
                          className={`h-11 text-base ${
                            inTol === false ? 'border-destructive focus-visible:ring-destructive'
                            : inTol === true ? 'border-green-500 focus-visible:ring-green-500' : ''
                          }`}
                          disabled={isDone}
                        />
                        {isQuant && val.trim() && (
                          <span className={`text-xl font-bold ${inTol ? 'text-green-600 dark:text-green-400' : 'text-destructive'}`}>
                            {inTol ? '✓' : '✗'}
                          </span>
                        )}
                      </div>
                      {measureErrors[c.id] && <p className="text-destructive text-xs">{measureErrors[c.id]}</p>}
                      {isQuant && val.trim() && inTol === false && (
                        <p className="text-destructive text-xs">Hors tolérance — vérifiez avant de terminer.</p>
                      )}
                    </div>
                  )
                })
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Material consumption */}
        <TabsContent value="material">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">Déclaration matière</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <div className="flex gap-2">
                <div className="flex-1">
                  <Label htmlFor="lot" className="sr-only">N° lot</Label>
                  <Input
                    id="lot"
                    placeholder="N° lot (scan ou saisie)"
                    value={lotId}
                    onChange={(e) => setLotId(e.target.value)}
                    className="h-11 text-base"
                  />
                </div>
                <div className="w-20">
                  <Label htmlFor="qty" className="sr-only">Quantité</Label>
                  <Input
                    id="qty"
                    type="number"
                    min="1"
                    placeholder="Qté"
                    value={qty}
                    onChange={(e) => setQty(e.target.value)}
                    className="h-11 text-base"
                  />
                </div>
              </div>
              <Button onClick={handleConsumeMaterial} disabled={busy || !lotId.trim()} className="h-11">
                Valider la consommation
              </Button>
              {materialMsg && (
                <p className={`text-sm ${materialMsg.ok ? 'text-green-600 dark:text-green-400' : 'text-destructive'}`}>
                  {materialMsg.text}
                </p>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Instructions */}
        {operation.instructions_url && (
          <TabsContent value="instructions">
            <Card>
              <CardContent className="p-4">
                <a
                  href={operation.instructions_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary underline text-sm break-all"
                >
                  {operation.instructions_url}
                </a>
              </CardContent>
            </Card>
          </TabsContent>
        )}
      </Tabs>

      {/* Suspend OF */}
      {isInProgress && (
        <div className="pt-2">
          <Separator className="mb-3" />
          {!showSuspend ? (
            <Button variant="outline" className="text-yellow-600 dark:text-yellow-400 border-yellow-300 dark:border-yellow-700 hover:bg-yellow-50 dark:hover:bg-yellow-950/30" onClick={() => setShowSuspend(true)}>
              <PauseOctagon className="size-4" />
              Suspendre l'OF
            </Button>
          ) : (
            <Card className="border-yellow-300 dark:border-yellow-700">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm text-yellow-700 dark:text-yellow-400">Suspendre l'OF</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-3">
                <Select value={suspendReason} onValueChange={(v) => setSuspendReason(v ?? '')}>
                  <SelectTrigger className="h-11">
                    <SelectValue placeholder="— Sélectionner un motif —" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="Attente matière">Attente matière</SelectItem>
                    <SelectItem value="Panne machine">Panne machine</SelectItem>
                    <SelectItem value="Décision qualité">Décision qualité</SelectItem>
                    <SelectItem value="Autre">Autre</SelectItem>
                  </SelectContent>
                </Select>
                <div className="flex gap-2">
                  <Button onClick={handleSuspend} disabled={busy || !suspendReason} variant="outline" className="flex-1 border-yellow-400 text-yellow-700 dark:text-yellow-400 hover:bg-yellow-50 dark:hover:bg-yellow-950/30">
                    Confirmer la suspension
                  </Button>
                  <Button variant="ghost" onClick={() => setShowSuspend(false)}>Annuler</Button>
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* Alert dialog — step 1: pick */}
      <Dialog open={alertOpen && !alertConfirm} onOpenChange={(open) => { if (!open) { setAlertOpen(false); setAlertCategory(null) } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Signaler un aléa</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">Sélectionnez la nature du problème :</p>
          <div className="grid grid-cols-2 gap-3">
            {ALERT_OPTIONS.map((opt) => (
              <button
                key={opt.category}
                onClick={() => { setAlertCategory(opt); setAlertConfirm(true) }}
                className="flex flex-col items-center gap-2 rounded-lg border-2 border-border hover:border-primary hover:bg-accent p-4 transition-all active:scale-95"
              >
                <opt.icon className={`size-8 ${opt.color}`} />
                <span className="text-sm font-medium text-center">{opt.label}</span>
              </button>
            ))}
          </div>
        </DialogContent>
      </Dialog>

      {/* Alert dialog — step 2: confirm */}
      <Dialog open={alertOpen && alertConfirm} onOpenChange={(open) => { if (!open) { setAlertConfirm(false); setAlertCategory(null) } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Confirmer l'aléa</DialogTitle>
          </DialogHeader>
          {alertCategory && (
            <div className="flex items-center gap-4 rounded-lg bg-muted p-4">
              <alertCategory.icon className={`size-10 ${alertCategory.color}`} />
              <div>
                <p className="font-semibold">{alertCategory.label}</p>
                <p className="text-sm text-muted-foreground">Opération : {operation.name}</p>
              </div>
            </div>
          )}
          <p className="text-sm text-muted-foreground">
            Un aléa sera envoyé au superviseur immédiatement.
          </p>
          <DialogFooter className="flex-col gap-2">
            <Button onClick={handleRaiseAlert} disabled={busy} variant="destructive" size="lg" className="w-full">
              Envoyer l'aléa
            </Button>
            <Button variant="ghost" className="w-full" onClick={() => setAlertConfirm(false)}>
              Retour
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
