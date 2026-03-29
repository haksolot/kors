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

interface Operation {
  id: string;
  of_id: string;
  step_number: number;
  name: string;
  status: 'OPERATION_STATUS_PENDING' | 'OPERATION_STATUS_IN_PROGRESS' | 'OPERATION_STATUS_COMPLETED';
}

export default function App() {
  const [token, setToken] = useState<string>(localStorage.getItem('jwt_token') || '')
  const [orders, setOrders] = useState<Order[]>([])
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null)
  const [operations, setOperations] = useState<Operation[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const saveToken = (newToken: string) => {
    setToken(newToken)
    localStorage.setItem('jwt_token', newToken)
  }

  const fetchDispatchList = async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/api/v1/dispatch', {
        headers: {
          'Authorization': `Bearer ${token.trim()}`,
          'Accept': 'application/json'
        }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `Error: ${res.status}`);
      setOrders(data.orders || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const fetchOperations = async (orderId: string) => {
    setLoading(true);
    try {
      const res = await fetch(`/api/v1/orders/${orderId}/operations`, {
        headers: {
          'Authorization': `Bearer ${token.trim()}`,
          'Accept': 'application/json'
        }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to fetch operations');
      setOperations(data.operations || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const startOperation = async (orderId: string, opId: string) => {
    try {
      const res = await fetch(`/api/v1/orders/${orderId}/operations/${opId}/start`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token.trim()}`,
          'Accept': 'application/json'
        }
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to start operation');
      }
      // Refresh operations after starting
      fetchOperations(orderId);
    } catch (err: any) {
      setError(err.message);
    }
  };

  useEffect(() => {
    if (token && !selectedOrder) {
      fetchDispatchList()
    }
  }, [token, selectedOrder])

  const selectOrder = (order: Order) => {
    setSelectedOrder(order);
    fetchOperations(order.id);
  };

  const getStatusBadgeVariant = (status: string) => {
    if (status.includes('PLANNED') || status.includes('PENDING')) return 'secondary';
    if (status.includes('IN_PROGRESS')) return 'default';
    if (status.includes('COMPLETED')) return 'outline';
    return 'destructive';
  }

  const formatStatus = (status: string) => {
    return status.replace('ORDER_STATUS_', '').replace('OPERATION_STATUS_', '').replace('_', ' ');
  }

  if (selectedOrder) {
    return (
      <div className="container mx-auto p-8 max-w-5xl space-y-8">
        <Button variant="ghost" onClick={() => setSelectedOrder(null)}>← Back to Dispatch List</Button>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">Order: {selectedOrder.reference}</h1>
            <p className="text-muted-foreground mt-2">Manage operations for this manufacturing order.</p>
          </div>
          <Badge variant={getStatusBadgeVariant(selectedOrder.status)} className="text-lg px-4 py-1">
            {formatStatus(selectedOrder.status)}
          </Badge>
        </div>

        {error && <div className="bg-destructive/15 text-destructive p-4 rounded-md">{error}</div>}

        <Card>
          <CardHeader>
            <CardTitle>Operations</CardTitle>
            <CardDescription>Follow the production sequence (Step Numbers).</CardDescription>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="text-center p-8 text-muted-foreground">Loading operations...</div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-20">Step</TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {operations.map((op) => (
                    <TableRow key={op.id}>
                      <TableCell className="font-mono">{op.step_number}</TableCell>
                      <TableCell className="font-medium">{op.name}</TableCell>
                      <TableCell>
                        <Badge variant={getStatusBadgeVariant(op.status)}>
                          {formatStatus(op.status)}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        {op.status.includes('PENDING') && (
                          <Button size="sm" onClick={() => startOperation(selectedOrder.id, op.id)}>Start</Button>
                        )}
                        {op.status.includes('IN_PROGRESS') && (
                          <Button size="sm" variant="outline" disabled>Complete (In Progress)</Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    );
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
                  <TableHead className="text-right">Actions</TableHead>
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
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" onClick={() => selectOrder(order)}>View Operations →</Button>
                    </TableCell>
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
