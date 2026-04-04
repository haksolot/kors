// Types matching proto JSON output (UseProtoNames: true → snake_case keys)

export type OrderStatus =
  | 'ORDER_STATUS_UNSPECIFIED'
  | 'ORDER_STATUS_PLANNED'
  | 'ORDER_STATUS_IN_PROGRESS'
  | 'ORDER_STATUS_COMPLETED'
  | 'ORDER_STATUS_SUSPENDED'
  | 'ORDER_STATUS_CANCELLED'

export type OperationStatus =
  | 'OPERATION_STATUS_UNSPECIFIED'
  | 'OPERATION_STATUS_PENDING'
  | 'OPERATION_STATUS_IN_PROGRESS'
  | 'OPERATION_STATUS_COMPLETED'
  | 'OPERATION_STATUS_SKIPPED'
  | 'OPERATION_STATUS_PENDING_SIGN_OFF'
  | 'OPERATION_STATUS_RELEASED'

export type AlertCategory =
  | 'ALERT_CATEGORY_MACHINE'
  | 'ALERT_CATEGORY_QUALITY'
  | 'ALERT_CATEGORY_PLANNING'
  | 'ALERT_CATEGORY_LOGISTICS'

export type CharacteristicType = 'CHARACTERISTIC_TYPE_QUANTITATIVE' | 'CHARACTERISTIC_TYPE_QUALITATIVE'

export interface ManufacturingOrder {
  id: string
  reference: string
  product_id: string
  quantity: number
  status: OrderStatus
  priority: number
  due_date?: string
  is_fai?: boolean
  created_at?: string
  updated_at?: string
  started_at?: string
  completed_at?: string
}

export interface Operation {
  id: string
  of_id: string
  step_number: number
  name: string
  operator_id?: string
  status: OperationStatus
  instructions_url?: string
  planned_duration_seconds?: number
  actual_duration_seconds?: number
  requires_sign_off?: boolean
  signed_off_by?: string
  required_skill?: string
  created_at?: string
  started_at?: string
  completed_at?: string
}

export interface ControlCharacteristic {
  id: string
  step_id: string
  name: string
  type: CharacteristicType
  unit?: string
  nominal_value?: number
  upper_tolerance?: number
  lower_tolerance?: number
  is_mandatory?: boolean
}

export interface Measurement {
  id: string
  operation_id: string
  characteristic_id: string
  value: string
  operator_id?: string
  is_ok?: boolean
  recorded_at?: string
}

export interface Alert {
  id: string
  category: AlertCategory
  status: string
  workstation_id?: string
  operation_id?: string
  message: string
  created_at?: string
}

export interface ApiError {
  error: string
}

// ── Admin — Qualifications ────────────────────────────────────────────────────

export type QualificationStatus =
  | 'QUALIFICATION_STATUS_UNSPECIFIED'
  | 'QUALIFICATION_STATUS_ACTIVE'
  | 'QUALIFICATION_STATUS_EXPIRING'
  | 'QUALIFICATION_STATUS_EXPIRED'
  | 'QUALIFICATION_STATUS_REVOKED'

export interface Qualification {
  id: string
  operator_id: string
  skill: string
  label?: string
  issued_at?: string
  expires_at?: string
  status: QualificationStatus
  granted_by?: string
  certificate_url?: string
  is_revoked?: boolean
  revoked_by?: string
  revoked_at?: string
  revoke_reason?: string
  created_at?: string
}

// ── Admin — Tools ─────────────────────────────────────────────────────────────

export type ToolStatus =
  | 'TOOL_STATUS_UNSPECIFIED'
  | 'TOOL_STATUS_VALID'
  | 'TOOL_STATUS_EXPIRED'
  | 'TOOL_STATUS_BLOCKED'
  | 'TOOL_STATUS_DECOMMISSIONED'

export interface Tool {
  id: string
  serial_number?: string
  name?: string
  description?: string
  category?: string
  status: ToolStatus
  last_calibration_at?: string
  next_calibration_at?: string
  current_cycles?: number
  max_cycles?: number
  created_at?: string
}

// ── Admin — Workstations ──────────────────────────────────────────────────────

export interface Workstation {
  id: string
  name: string
  description?: string
  capacity?: number
  nominal_rate?: number
  status: WorkstationStatus
  created_at?: string
  updated_at?: string
}

// ── Admin — Routings ──────────────────────────────────────────────────────────

export interface RoutingStep {
  id: string
  routing_id: string
  step_number: number
  name: string
  planned_duration_seconds?: number
  required_skill?: string
  instructions_url?: string
  requires_sign_off?: boolean
}

export interface Routing {
  id: string
  product_id: string
  version: number
  name: string
  is_active?: boolean
  steps: RoutingStep[]
  created_at?: string
}

// ── QMS ───────────────────────────────────────────────────────────────────────

export type NCStatus =
  | 'NC_STATUS_UNSPECIFIED'
  | 'NC_STATUS_OPEN'
  | 'NC_STATUS_UNDER_ANALYSIS'
  | 'NC_STATUS_PENDING_DISPOSITION'
  | 'NC_STATUS_CLOSED'

export type NCDisposition =
  | 'NC_DISPOSITION_UNSPECIFIED'
  | 'NC_DISPOSITION_REWORK'
  | 'NC_DISPOSITION_SCRAP'
  | 'NC_DISPOSITION_USE_AS_IS'
  | 'NC_DISPOSITION_RETURN_TO_SUPPLIER'

export type CAPAStatus =
  | 'CAPA_STATUS_UNSPECIFIED'
  | 'CAPA_STATUS_OPEN'
  | 'CAPA_STATUS_IN_PROGRESS'
  | 'CAPA_STATUS_COMPLETED'
  | 'CAPA_STATUS_CANCELLED'

export type CAPAActionType =
  | 'CAPA_ACTION_TYPE_UNSPECIFIED'
  | 'CAPA_ACTION_TYPE_CORRECTIVE'
  | 'CAPA_ACTION_TYPE_PREVENTIVE'

export interface NonConformity {
  id: string
  operation_id?: string
  of_id?: string
  defect_code?: string
  description?: string
  affected_quantity?: number
  serial_numbers?: string[]
  declared_by?: string
  status: NCStatus
  disposition?: NCDisposition
  closed_by?: string
  created_at?: string
  updated_at?: string
  closed_at?: string
}

export interface CAPA {
  id: string
  nc_id: string
  action_type: CAPAActionType
  description?: string
  owner_id?: string
  status: CAPAStatus
  due_date?: string
  created_at?: string
  updated_at?: string
  completed_at?: string
}

// ── Compliance / Audit ────────────────────────────────────────────────────────

export interface AuditEntry {
  id: string
  actor_id?: string
  actor_role?: string
  action?: string
  entity_type?: string
  entity_id?: string
  workstation_id?: string
  notes?: string
  created_at?: string
}

export interface AsBuiltMeasurement {
  characteristic_id: string
  value: string
  status: string
  operator_id?: string
  recorded_at?: string
}

export interface AsBuiltConsumedLot {
  lot_id: string
  quantity: number
}

export interface AsBuiltTool {
  tool_id: string
  serial_number?: string
  name?: string
  calibration_expiry?: string
}

export interface AsBuiltOperation {
  operation_id: string
  step_number: number
  name: string
  status: string
  operator_id?: string
  workstation_id?: string
  requires_sign_off?: boolean
  signed_off_by?: string
  signed_off_at?: string
  is_special_process?: boolean
  nadcap_process_code?: string
  planned_duration_seconds?: number
  actual_duration_seconds?: number
  started_at?: string
  completed_at?: string
  measurements: AsBuiltMeasurement[]
  consumed_lots: AsBuiltConsumedLot[]
  tools: AsBuiltTool[]
}

export interface AsBuiltReport {
  generated_at?: string
  order_id: string
  reference: string
  product_id?: string
  quantity: number
  status: string
  is_fai?: boolean
  fai_approved_by?: string
  fai_approved_at?: string
  started_at?: string
  completed_at?: string
  operations: AsBuiltOperation[]
  serial_numbers: unknown[]
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export type WorkstationStatus =
  | 'WORKSTATION_STATUS_UNSPECIFIED'
  | 'WORKSTATION_STATUS_AVAILABLE'
  | 'WORKSTATION_STATUS_IN_PRODUCTION'
  | 'WORKSTATION_STATUS_DOWN'
  | 'WORKSTATION_STATUS_MAINTENANCE'

export interface WorkstationSnapshot {
  workstation_id: string
  workstation_name: string
  status: WorkstationStatus
  current_of_id?: string
  current_of_reference?: string
  trs?: number
  availability?: number
  performance?: number
  quality?: number
}

export interface SupervisorDashboard {
  active_orders: ManufacturingOrder[]
  workstations: WorkstationSnapshot[]
  active_alerts: Alert[]
}

export interface TRSDataPoint {
  period: string
  trs: number
  availability: number
  performance: number
  quality: number
}

export interface DowntimeCause {
  reason: string
  total_duration_seconds: number
  occurrence_count: number
}

export interface ProductionProgressLine {
  of_id: string
  of_reference: string
  product_id: string
  planned_quantity: number
  good_quantity: number
  scrap_quantity: number
  completion_percentage: number
}
