import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, ChevronDown, ChevronRight, AlertCircle, Clock, Star, ExternalLink } from 'lucide-react'
import { listRoutings, getRouting } from '@/lib/api'
import type { Routing, RoutingStep } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

function formatDuration(s?: number): string {
  if (!s) return '—'
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (h > 0) return `${h}h${m > 0 ? m + 'min' : ''}`
  return `${m}min`
}

function StepRow({ step }: { step: RoutingStep }) {
  return (
    <div className="flex items-start gap-3 py-2.5">
      <div className="size-6 rounded-full bg-muted flex items-center justify-center text-xs font-bold text-muted-foreground shrink-0 mt-0.5">
        {step.step_number}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium">{step.name}</p>
        <div className="flex flex-wrap gap-3 text-xs text-muted-foreground mt-0.5">
          {step.planned_duration_seconds != null && step.planned_duration_seconds > 0 && (
            <span className="flex items-center gap-1">
              <Clock className="size-3" />
              {formatDuration(step.planned_duration_seconds)}
            </span>
          )}
          {step.required_skill && (
            <span className="flex items-center gap-1">
              <Star className="size-3" />
              <span className="text-primary">{step.required_skill}</span>
            </span>
          )}
          {step.requires_sign_off && (
            <Badge className="bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border-0 text-xs">
              Visa qualité
            </Badge>
          )}
          {step.instructions_url && (
            <a href={step.instructions_url} target="_blank" rel="noopener noreferrer"
              className="text-primary underline flex items-center gap-1">
              <ExternalLink className="size-3" />
              Instructions
            </a>
          )}
        </div>
      </div>
    </div>
  )
}

export default function RoutingsPage() {
  const [routings, setRoutings] = useState<Routing[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [detail, setDetail] = useState<Record<string, Routing>>({})
  const [loadingDetail, setLoadingDetail] = useState<string | null>(null)
  const [filter, setFilter] = useState<'all' | 'active' | 'inactive'>('active')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await listRoutings()
      setRoutings(res.routings ?? [])
      setError(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erreur')
    } finally { setLoading(false) }
  }, [])

  useEffect(() => { load() }, [load])

  async function toggleExpand(id: string) {
    if (expanded === id) { setExpanded(null); return }
    setExpanded(id)
    if (detail[id]) return
    setLoadingDetail(id)
    try {
      const res = await getRouting(id)
      setDetail((prev) => ({ ...prev, [id]: res.routing }))
    } catch {
      // steps won't load but header stays visible
    } finally {
      setLoadingDetail(null)
    }
  }

  const displayed = routings.filter((r) => {
    if (filter === 'active') return r.is_active
    if (filter === 'inactive') return !r.is_active
    return true
  })

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">Gammes opératoires</h1>
          <p className="text-sm text-muted-foreground">{routings.length} gamme(s) configurée(s)</p>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      <Tabs value={filter} onValueChange={(v) => setFilter(v as 'all' | 'active' | 'inactive')}>
        <TabsList>
          <TabsTrigger value="active">Actives ({routings.filter((r) => r.is_active).length})</TabsTrigger>
          <TabsTrigger value="inactive">Inactives ({routings.filter((r) => !r.is_active).length})</TabsTrigger>
          <TabsTrigger value="all">Toutes ({routings.length})</TabsTrigger>
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
        <div className="flex flex-col gap-3">
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
        </div>
      ) : displayed.length === 0 ? (
        <Card><CardContent className="p-8 text-center text-muted-foreground">Aucune gamme dans cette vue.</CardContent></Card>
      ) : (
        <div className="flex flex-col gap-2">
          {displayed.map((routing) => {
            const isOpen = expanded === routing.id
            const steps = detail[routing.id]?.steps ?? []
            return (
              <Card key={routing.id} className="overflow-hidden">
                <button
                  onClick={() => toggleExpand(routing.id)}
                  className="w-full text-left px-4 py-3 flex items-center gap-3 hover:bg-accent transition-colors"
                >
                  {isOpen ? <ChevronDown className="size-4 text-muted-foreground shrink-0" /> : <ChevronRight className="size-4 text-muted-foreground shrink-0" />}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <p className="font-semibold">{routing.name}</p>
                      <span className="text-xs text-muted-foreground">v{routing.version}</span>
                      <span className="text-xs text-muted-foreground font-mono">{routing.product_id.slice(0, 8)}…</span>
                    </div>
                  </div>
                  <Badge className={routing.is_active
                    ? 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0'
                    : 'bg-secondary text-secondary-foreground border-0'
                  }>
                    {routing.is_active ? 'Active' : 'Inactive'}
                  </Badge>
                </button>

                {isOpen && (
                  <div className="border-t bg-muted/30 px-4 pb-3">
                    {loadingDetail === routing.id ? (
                      <div className="py-3 flex flex-col gap-2">
                        <Skeleton className="h-6 w-full" />
                        <Skeleton className="h-6 w-3/4" />
                      </div>
                    ) : steps.length === 0 ? (
                      <p className="text-sm text-muted-foreground py-3">Aucune étape définie pour cette gamme.</p>
                    ) : (
                      <div className="divide-y">
                        {steps.map((step) => <StepRow key={step.id} step={step} />)}
                      </div>
                    )}
                  </div>
                )}
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
