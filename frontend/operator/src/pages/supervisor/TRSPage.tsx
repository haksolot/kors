import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { fetchTRS, fetchDowntimeCauses } from '@/lib/api'
import type { TRSDataPoint, DowntimeCause } from '@/lib/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip,
  Legend, ResponsiveContainer, BarChart, Bar, Cell,
} from 'recharts'

type Granularity = 'DAY' | 'WEEK' | 'MONTH'

function isoDateMinus(days: number) {
  const d = new Date()
  d.setDate(d.getDate() - days)
  return d.toISOString()
}

function formatPeriod(period: string, granularity: Granularity): string {
  const d = new Date(period)
  if (granularity === 'MONTH') return d.toLocaleDateString('fr-FR', { month: 'short', year: '2-digit' })
  if (granularity === 'WEEK') {
    const jan1 = new Date(d.getFullYear(), 0, 1)
    return `S${Math.ceil(((d.getTime() - jan1.getTime()) / 86400000 + jan1.getDay() + 1) / 7)}`
  }
  return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
}

const DOWNTIME_COLORS = ['#ef4444', '#f97316', '#eab308', '#22c55e', '#3b82f6', '#a855f7', '#ec4899']

export default function TRSPage() {
  const [granularity, setGranularity] = useState<Granularity>('DAY')
  const [points, setPoints] = useState<TRSDataPoint[]>([])
  const [causes, setCauses] = useState<DowntimeCause[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  function load(g: Granularity = granularity) {
    const days = g === 'DAY' ? 14 : g === 'WEEK' ? 56 : 180
    const from = isoDateMinus(days)
    const to = new Date().toISOString()
    setLoading(true)
    Promise.all([fetchTRS({ from, to, granularity: g }), fetchDowntimeCauses(from, to)])
      .then(([trsRes, causeRes]) => {
        setPoints(trsRes.points ?? [])
        setCauses((causeRes.causes ?? []).sort((a, b) => b.total_duration_seconds - a.total_duration_seconds))
        setError(null)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Erreur'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [granularity])

  const chartData = points.map((p) => ({
    label: formatPeriod(p.period, granularity),
    TRS: Math.round(p.trs * 100),
    Disponibilité: Math.round(p.availability * 100),
    Performance: Math.round(p.performance * 100),
    Qualité: Math.round(p.quality * 100),
  }))

  const avgTRS = points.length
    ? Math.round((points.reduce((s, p) => s + p.trs, 0) / points.length) * 100)
    : null

  const trsColorClass = avgTRS == null ? '' : avgTRS >= 85 ? 'text-green-600 dark:text-green-400' : avgTRS >= 65 ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400'

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-xl font-bold">TRS / OEE</h1>
          <p className="text-sm text-muted-foreground">Taux de rendement synthétique par période</p>
        </div>
        <div className="flex items-center gap-3">
          {avgTRS !== null && (
            <span className="text-sm text-muted-foreground">
              Moy. TRS : <span className={`font-bold text-lg ${trsColorClass}`}>{avgTRS} %</span>
            </span>
          )}
          <Button variant="outline" size="sm" onClick={() => load()} disabled={loading}>
            <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
          </Button>
        </div>
      </div>

      <Tabs value={granularity} onValueChange={(v) => setGranularity(v as Granularity)}>
        <TabsList>
          <TabsTrigger value="DAY">Jour</TabsTrigger>
          <TabsTrigger value="WEEK">Semaine</TabsTrigger>
          <TabsTrigger value="MONTH">Mois</TabsTrigger>
        </TabsList>
      </Tabs>

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-3 text-destructive text-sm">{error}</CardContent>
        </Card>
      )}

      {loading ? (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-72 w-full" />
          <Skeleton className="h-48 w-full" />
        </div>
      ) : (
        <>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                TRS — Disponibilité × Performance × Qualité
              </CardTitle>
            </CardHeader>
            <CardContent>
              {chartData.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-8">Pas de données pour cette période.</p>
              ) : (
                <ResponsiveContainer width="100%" height={280}>
                  <LineChart data={chartData} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                    <XAxis dataKey="label" tick={{ fontSize: 11 }} className="fill-muted-foreground" />
                    <YAxis domain={[0, 100]} tick={{ fontSize: 11 }} unit="%" />
                    <Tooltip formatter={(v) => `${v} %`} />
                    <Legend />
                    <Line type="monotone" dataKey="TRS" stroke="#3b82f6" strokeWidth={2} dot={false} />
                    <Line type="monotone" dataKey="Disponibilité" stroke="#22c55e" strokeWidth={1.5} strokeDasharray="4 4" dot={false} />
                    <Line type="monotone" dataKey="Performance" stroke="#f97316" strokeWidth={1.5} strokeDasharray="4 4" dot={false} />
                    <Line type="monotone" dataKey="Qualité" stroke="#a855f7" strokeWidth={1.5} strokeDasharray="4 4" dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>

          {causes.length > 0 && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">Causes d'arrêt (durée cumulée)</CardTitle>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={220}>
                  <BarChart data={causes.slice(0, 7).map((c) => ({
                    name: c.reason,
                    heures: Math.round(c.total_duration_seconds / 3600 * 10) / 10,
                  }))} layout="vertical" margin={{ top: 0, right: 20, left: 0, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                    <XAxis type="number" tick={{ fontSize: 11 }} unit="h" />
                    <YAxis type="category" dataKey="name" tick={{ fontSize: 11 }} width={90} />
                    <Tooltip formatter={(v) => `${v} h`} />
                    <Bar dataKey="heures" radius={[0, 4, 4, 0]}>
                      {causes.slice(0, 7).map((_, i) => (
                        <Cell key={i} fill={DOWNTIME_COLORS[i % DOWNTIME_COLORS.length]} />
                      ))}
                    </Bar>
                  </BarChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  )
}
