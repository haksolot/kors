import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

type OrderStatus = 'ORDER_STATUS_UNSPECIFIED' | 'ORDER_STATUS_PLANNED' | 'ORDER_STATUS_IN_PROGRESS' | 'ORDER_STATUS_COMPLETED' | 'ORDER_STATUS_SUSPENDED' | 'ORDER_STATUS_CANCELLED';

interface Order {
  id: string;
  reference: string;
  product_id: string;
  quantity: number;
  status: OrderStatus;
  priority: number;
}

export default function App() {
  const [token, setToken] = useState<string>(localStorage.getItem('jwt_token') || '')
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const saveToken = (newToken: string) => {
    setToken(newToken)
    localStorage.setItem('jwt_token', newToken)
  }

  const fetchDispatchList = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/v1/dispatch?limit=50', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      if (!res.ok) {
        throw new Error(`Error: ${res.status} ${res.statusText}`)
      }
      const data = await res.json()
      setOrders(data.orders || [])
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (token) {
      fetchDispatchList()
    }
  }, [token])

  const getStatusBadgeVariant = (status: OrderStatus) => {
    switch(status) {
      case 'ORDER_STATUS_PLANNED': return 'secondary';
      case 'ORDER_STATUS_IN_PROGRESS': return 'default';
      case 'ORDER_STATUS_COMPLETED': return 'outline';
      case 'ORDER_STATUS_SUSPENDED': return 'destructive';
      default: return 'outline';
    }
  }

  const formatStatus = (status: OrderStatus) => {
    return status.replace('ORDER_STATUS_', '').replace('_', ' ');
  }

  return (
    <div className="container mx-auto p-8 max-w-5xl space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">KORS MES Operator Panel</h1>
          <p className="text-muted-foreground mt-2">Manage manufacturing operations and dispatch lists.</p>
        </div>
      </div>

      <Card className="bg-muted/40">
        <CardContent className="p-4 flex gap-4">
          <input 
            type="text" 
            placeholder="Paste JWT Token here..." 
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            value={token}
            onChange={(e) => saveToken(e.target.value)}
          />
          <Button onClick={fetchDispatchList} disabled={loading}>Refresh</Button>
        </CardContent>
      </Card>

      {error && (
        <div className="bg-destructive/15 text-destructive p-4 rounded-md">
          {error}
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Dispatch List</CardTitle>
          <CardDescription>Orders ready or in progress.</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="text-center p-8 text-muted-foreground">Loading...</div>
          ) : orders.length === 0 ? (
            <div className="text-center p-8 text-muted-foreground">No orders found. Ensure your token is valid and backend is running.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Reference</TableHead>
                  <TableHead>Product ID</TableHead>
                  <TableHead>Quantity</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Priority</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orders.map((order) => (
                  <TableRow key={order.id}>
                    <TableCell className="font-medium">{order.reference}</TableCell>
                    <TableCell className="font-mono text-xs">{order.product_id}</TableCell>
                    <TableCell>{order.quantity}</TableCell>
                    <TableCell>
                      <Badge variant={getStatusBadgeVariant(order.status)}>
                        {formatStatus(order.status)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">{order.priority}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
