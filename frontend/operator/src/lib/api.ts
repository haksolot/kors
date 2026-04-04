import type {
  ManufacturingOrder,
  Operation,
  ControlCharacteristic,
  Measurement,
  Alert,
  NonConformity, CAPA, NCDisposition,
  AuditEntry, AsBuiltReport,
  SupervisorDashboard, TRSDataPoint, DowntimeCause, ProductionProgressLine,
  Qualification, Tool, Workstation, Routing,
} from './types'

const BASE = '/api/v1'

function getToken(): string {
  return localStorage.getItem('jwt_token') ?? ''
}

function authHeaders(): HeadersInit {
  return {
    Authorization: `Bearer ${getToken()}`,
    'Content-Type': 'application/json',
    Accept: 'application/json',
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: { ...authHeaders(), ...init?.headers },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error((body as { error?: string }).error ?? res.statusText)
  }
  return res.json() as Promise<T>
}

// ── Orders ────────────────────────────────────────────────────────────────────

export function fetchDispatch(limit = 50): Promise<{ orders: ManufacturingOrder[] }> {
  return request(`/dispatch?limit=${limit}`)
}

export function fetchOrder(id: string): Promise<{ order: ManufacturingOrder }> {
  return request(`/orders/${id}`)
}

export function suspendOrder(id: string, reason: string): Promise<{ order: ManufacturingOrder }> {
  return request(`/orders/${id}/suspend`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  })
}

export function resumeOrder(id: string): Promise<{ order: ManufacturingOrder }> {
  return request(`/orders/${id}/resume`, { method: 'POST' })
}

// ── Operations ────────────────────────────────────────────────────────────────

export function fetchOperations(ofId: string): Promise<{ operations: Operation[] }> {
  return request(`/orders/${ofId}/operations`)
}

export function fetchOperation(ofId: string, opId: string): Promise<{ operation: Operation }> {
  return request(`/orders/${ofId}/operations/${opId}`)
}

export function startOperation(ofId: string, opId: string): Promise<{ operation: Operation }> {
  return request(`/orders/${ofId}/operations/${opId}/start`, { method: 'POST' })
}

export function completeOperation(ofId: string, opId: string): Promise<{ operation: Operation }> {
  return request(`/orders/${ofId}/operations/${opId}/complete`, { method: 'POST' })
}

// ── Quality measurements ──────────────────────────────────────────────────────

export function fetchCharacteristics(opId: string): Promise<{ characteristics: ControlCharacteristic[] }> {
  return request(`/quality/operations/${opId}/characteristics`)
}

export function recordMeasurement(
  operationId: string,
  characteristicId: string,
  value: string,
): Promise<{ measurement: Measurement }> {
  return request('/quality/measurements', {
    method: 'POST',
    body: JSON.stringify({ operation_id: operationId, characteristic_id: characteristicId, value }),
  })
}

// ── Material consumption ──────────────────────────────────────────────────────

export function consumeMaterial(
  operationId: string,
  lotId: string,
  quantity: number,
): Promise<unknown> {
  return request('/materials/consume', {
    method: 'POST',
    body: JSON.stringify({ lot_id: lotId, operation_id: operationId, quantity }),
  })
}

// ── Time tracking ─────────────────────────────────────────────────────────────

export function recordTimeLog(
  ofId: string,
  operationId: string,
  durationSeconds: number,
): Promise<unknown> {
  return request('/time-logs', {
    method: 'POST',
    body: JSON.stringify({
      of_id: ofId,
      operation_id: operationId,
      type: 'TIME_LOG_TYPE_RUN',
      duration_seconds: durationSeconds,
    }),
  })
}

// ── Alerts ────────────────────────────────────────────────────────────────────

// ── QMS — Non-conformities ────────────────────────────────────────────────────

export function listNCs(): Promise<{ ncs: NonConformity[] }> {
  return request('/qms/nc')
}

export function getNC(id: string): Promise<{ nc: NonConformity }> {
  return request(`/qms/nc/${id}`)
}

export function startAnalysis(id: string): Promise<{ nc: NonConformity }> {
  return request(`/qms/nc/${id}/analyse`, { method: 'POST' })
}

export function proposeDisposition(id: string, disposition: NCDisposition): Promise<{ nc: NonConformity }> {
  return request(`/qms/nc/${id}/disposition`, {
    method: 'POST',
    body: JSON.stringify({ disposition }),
  })
}

export function closeNC(id: string): Promise<{ nc: NonConformity }> {
  return request(`/qms/nc/${id}/close`, { method: 'POST' })
}

// ── QMS — CAPA ────────────────────────────────────────────────────────────────

export function listCAPAs(ncId?: string): Promise<{ capas: CAPA[] }> {
  const body = ncId ? JSON.stringify({ nc_id: ncId }) : undefined
  return request('/qms/capa', { method: 'GET', body })
}

export function startCAPAAction(id: string): Promise<{ capa: CAPA }> {
  return request(`/qms/capa/${id}/start`, { method: 'POST' })
}

export function completeCAPA(id: string): Promise<{ capa: CAPA }> {
  return request(`/qms/capa/${id}/complete`, { method: 'POST' })
}

// ── Compliance ────────────────────────────────────────────────────────────────

export function getAsBuilt(ofId: string): Promise<AsBuiltReport> {
  return request(`/compliance/orders/${ofId}/as-built`)
}

export function queryAuditTrail(params: {
  actorId?: string
  entityType?: string
  entityId?: string
  action?: string
  from?: string
  to?: string
}): Promise<{ entries: AuditEntry[] }> {
  const q = new URLSearchParams()
  if (params.actorId) q.set('actor_id', params.actorId)
  if (params.entityType) q.set('entity_type', params.entityType)
  if (params.entityId) q.set('entity_id', params.entityId)
  if (params.action) q.set('action', params.action)
  if (params.from) q.set('from', params.from)
  if (params.to) q.set('to', params.to)
  return request(`/compliance/audit?${q}`)
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export function fetchSupervisorDashboard(): Promise<SupervisorDashboard> {
  return request('/dashboard/supervisor')
}

export function fetchTRS(params: {
  workstationId?: string
  from?: string
  to?: string
  granularity?: 'DAY' | 'WEEK' | 'MONTH'
}): Promise<{ points: TRSDataPoint[] }> {
  const q = new URLSearchParams()
  if (params.workstationId) q.set('workstation_id', params.workstationId)
  if (params.from) q.set('from', params.from)
  if (params.to) q.set('to', params.to)
  if (params.granularity) q.set('granularity', params.granularity)
  return request(`/dashboard/trs?${q}`)
}

export function fetchDowntimeCauses(from?: string, to?: string): Promise<{ causes: DowntimeCause[] }> {
  const q = new URLSearchParams()
  if (from) q.set('from', from)
  if (to) q.set('to', to)
  return request(`/dashboard/downtime-causes?${q}`)
}

export function fetchProductionProgress(from?: string, to?: string): Promise<{ lines: ProductionProgressLine[] }> {
  const q = new URLSearchParams()
  if (from) q.set('from', from)
  if (to) q.set('to', to)
  return request(`/dashboard/production-progress?${q}`)
}

export function acknowledgeAlert(id: string): Promise<unknown> {
  return request(`/alerts/${id}/acknowledge`, { method: 'POST' })
}

export function resolveAlert(id: string, notes: string): Promise<unknown> {
  return request(`/alerts/${id}/resolve`, {
    method: 'POST',
    body: JSON.stringify({ resolution_notes: notes }),
  })
}

// ── Alerts (raise) ────────────────────────────────────────────────────────────

// ── Admin — Qualifications ────────────────────────────────────────────────────

export function listExpiringQualifications(days = 30): Promise<{ qualifications: Qualification[] }> {
  return request(`/qualifications/expiring?days=${days}`)
}

export function listQualificationsByOperator(operatorId: string): Promise<{ qualifications: Qualification[] }> {
  return request(`/qualifications?operator_id=${encodeURIComponent(operatorId)}`)
}

export function renewQualification(id: string, newExpiresAt: string): Promise<{ qualification: Qualification }> {
  return request(`/qualifications/${id}/renew`, {
    method: 'POST',
    body: JSON.stringify({ new_expires_at: newExpiresAt }),
  })
}

export function revokeQualification(id: string, reason: string): Promise<{ qualification: Qualification }> {
  return request(`/qualifications/${id}/revoke`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  })
}

// ── Admin — Tools ─────────────────────────────────────────────────────────────

export function listTools(): Promise<{ tools: Tool[]; total: number }> {
  return request('/tools?limit=100')
}

export function calibrateTool(id: string, lastCalibration: string, nextCalibration: string): Promise<{ tool: Tool }> {
  return request(`/tools/${id}/calibrate`, {
    method: 'POST',
    body: JSON.stringify({ last_calibration_at: lastCalibration, next_calibration_at: nextCalibration }),
  })
}

// ── Admin — Workstations ──────────────────────────────────────────────────────

export function listWorkstations(): Promise<{ workstations: Workstation[]; total: number }> {
  return request('/workstations?limit=100')
}

export function createWorkstation(name: string, description: string, capacity: number, nominalRate: number): Promise<{ workstation: Workstation }> {
  return request('/workstations', {
    method: 'POST',
    body: JSON.stringify({ name, description, capacity, nominal_rate: nominalRate }),
  })
}

export function updateWorkstationStatus(id: string, status: string): Promise<{ workstation: Workstation }> {
  return request(`/workstations/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}

// ── Admin — Routings ──────────────────────────────────────────────────────────

export function listRoutings(): Promise<{ routings: Routing[] }> {
  return request('/routings')
}

export function getRouting(id: string): Promise<{ routing: Routing }> {
  return request(`/routings/${id}`)
}

// ── Alerts (raise) ────────────────────────────────────────────────────────────

export function raiseAlert(
  category: string,
  operationId: string,
  message: string,
): Promise<{ alert: Alert }> {
  return request('/alerts', {
    method: 'POST',
    body: JSON.stringify({ category, operation_id: operationId, message }),
  })
}
