import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { fetchProductionProgress } from '@/lib/api'
import type { ProductionProgressLine } from '@/lib/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Progress } from '@/components/ui/progress'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function isoDateMinus(days: number) {
  const d = new Date()
  d.setDate(d.getDate() - days)
  return d.toISOString()
}

function progressColor(pct: number): string {
  if (pct >= 100) return 'text-green-600 dark:text-green-400'
  if (pct >= 50)  return 'text-blue-600 dark:text-blue-400'
  return 'text-yellow-600 dark:text-yellow-400'
}

export default function ProgressPage() {
  const [lines, setLines] = useState<ProductionProgressLine[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [days, setDays] = useState(7)

  function load(d = days) {
    setLoading(true)
    fetchProductionProgress(isoDateMinus(d), new Date().toISOString())
      .then((res) => { setLines(res.lines ?? []); setError(null) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Erreur'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [days])

  const totalPlanned = lines.reduce((s, l) => s + l.planned_quantity, 0)
  const totalGood    = lines.reduce((s, l) => s + l.good_quantity, 0)
  const totalScrap   = lines.reduce((s, l) => s + l.scrap_quantity, 0)
  const globalPct    = totalPlanned > 0 ? Math.round((totalGood / totalPlanned) * 100) : 0

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-xl font-bold">Avancement production</h1>
          <p className="text-sm text-muted-foreground">Suivi quantitatif des ordres de fabrication</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => load()} disabled={loading}>
          <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
          Actualiser
        </Button>
      </div>

      <Tabs value={String(days)} onValueChange={(v) => setDays(Number(v))}>
        <TabsList>
          <TabsTrigger value="7">7 j</TabsTrigger>
          <TabsTrigger value="14">14 j</TabsTrigger>
          <TabsTrigger value="30">30 j</TabsTrigger>
        </TabsList>
      </Tabs>

      {error && (
        <Card className="border-destructive">
          <CardContent className="p-3 text-destructive text-sm">{error}</CardContent>
        </Card>
      )}

      {loading ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-48 w-full" />
        </div>
      ) : (
        <>
          {/* Global KPIs */}
          <div className="grid grid-cols-3 gap-4">
            <Card>
              <CardContent className="p-4 text-center">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Planifié</p>
                <p className="text-2xl font-bold mt-1">{totalPlanned}</p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4 text-center">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Produit</p>
                <p className="text-2xl font-bold mt-1 text-green-600 dark:text-green-400">{totalGood}</p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4 text-center">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Rebut</p>
                <p className={`text-2xl font-bold mt-1 ${totalScrap > 0 ? 'text-red-600 dark:text-red-400' : ''}`}>
                  {totalScrap}
                </p>
              </CardContent>
            </Card>
          </div>

          {/* Global progress */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground flex items-center justify-between">
                Avancement global
                <span className={`text-base font-bold ${progressColor(globalPct)}`}>{globalPct} %</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <Progress value={globalPct} className="h-3" />
            </CardContent>
          </Card>

          {/* Detail table */}
          {lines.length === 0 ? (
            <Card>
              <CardContent className="p-8 text-center text-muted-foreground">
                Aucun OF sur cette période.
              </CardContent>
            </Card>
          ) : (
            <Card>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Référence OF</TableHead>
                    <TableHead className="text-right">Planifié</TableHead>
                    <TableHead className="text-right">Produit</TableHead>
                    <TableHead className="text-right">Rebut</TableHead>
                    <TableHead className="w-40">Avancement</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {lines.map((l) => {
                    const pct = Math.round((l.completion_percentage ?? 0) * 100)
                    return (
                      <TableRow key={l.of_id}>
                        <TableCell className="font-medium">{l.of_reference}</TableCell>
                        <TableCell className="text-right text-muted-foreground">{l.planned_quantity}</TableCell>
                        <TableCell className="text-right font-medium text-green-600 dark:text-green-400">{l.good_quantity}</TableCell>
                        <TableCell className={`text-right font-medium ${l.scrap_quantity > 0 ? 'text-red-600 dark:text-red-400' : 'text-muted-foreground'}`}>
                          {l.scrap_quantity}
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <Progress value={Math.min(pct, 100)} className="h-2 flex-1" />
                            <span className={`text-xs font-medium w-10 text-right ${progressColor(pct)}`}>
                              {pct} %
                            </span>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </Card>
          )}
        </>
      )}
    </div>
  )
}
