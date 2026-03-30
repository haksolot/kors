package domain

// NATS subject constants for the MES domain.
// Format: kors.{domain}.{entity}.{past_tense_verb} (ADR-002)
// NEVER hardcode these strings in handlers — always use these constants.
const (
	// Async events (JetStream pub/sub)
	SubjectOFCreated          = "kors.mes.of.created"
	SubjectOFStarted          = "kors.mes.of.started"
	SubjectOFCompleted        = "kors.mes.of.completed"
	SubjectOFSuspended        = "kors.mes.of.suspended"
	SubjectOFResumed          = "kors.mes.of.resumed"
	SubjectOFCancelled        = "kors.mes.of.cancelled"
	SubjectOperationStarted   = "kors.mes.operation.started"
	SubjectOperationCompleted = "kors.mes.operation.completed"
	SubjectOperationSkipped   = "kors.mes.operation.skipped"

	// Synchronous request-reply subjects — orders
	SubjectOFCreate  = "kors.mes.of.create"
	SubjectOFGet     = "kors.mes.of.get"
	SubjectOFList    = "kors.mes.of.list"
	SubjectOFSuspend = "kors.mes.of.suspend"
	SubjectOFResume  = "kors.mes.of.resume"
	SubjectOFCancel  = "kors.mes.of.cancel"

	// Synchronous request-reply subjects — operations
	SubjectOperationCreate   = "kors.mes.operation.create"
	SubjectOperationStart    = "kors.mes.operation.start"
	SubjectOperationComplete = "kors.mes.operation.complete"
	SubjectOperationSkip     = "kors.mes.operation.skip"
	SubjectOperationGet      = "kors.mes.operation.get"
	SubjectOperationList     = "kors.mes.operation.list"

	// Synchronous request-reply subjects — traceability (lots & serial numbers)
	SubjectLotCreate  = "kors.mes.lot.create"
	SubjectLotGet     = "kors.mes.lot.get"
	SubjectSNRegister = "kors.mes.sn.register"
	SubjectSNRelease  = "kors.mes.sn.release"
	SubjectSNScrap    = "kors.mes.sn.scrap"
	SubjectSNGet      = "kors.mes.sn.get"

	// Synchronous request-reply subjects — genealogy
	SubjectGenealogyAdd = "kors.mes.genealogy.add"
	SubjectGenealogyGet = "kors.mes.genealogy.get"

	// Async events — traceability
	SubjectLotCreated          = "kors.mes.lot.created"
	SubjectSNReleased          = "kors.mes.sn.released"
	SubjectSNScrapped          = "kors.mes.sn.scrapped"
	SubjectGenealogyEntryAdded = "kors.mes.genealogy.entry_added"

	// Synchronous request-reply subjects — quality (BLOC 4)
	SubjectOperationSignOff         = "kors.mes.operation.sign_off"
	SubjectOperationDeclareNC       = "kors.mes.operation.declare_nc"
	SubjectOperationAttachInstructions = "kors.mes.operation.attach_instructions"
	SubjectOFFAIApprove             = "kors.mes.of.fai_approve"

	// Async events — quality (BLOC 4)
	SubjectOperationSignedOff = "kors.mes.operation.signed_off"
	SubjectNCDeclared         = "kors.mes.nc.declared"
	SubjectOFFAIApproved      = "kors.mes.of.fai_approved"

	// Synchronous request-reply subjects — routings (BLOC 5)
	SubjectRoutingCreate       = "kors.mes.routing.create"
	SubjectRoutingGet          = "kors.mes.routing.get"
	SubjectRoutingList         = "kors.mes.routing.list"
	SubjectOFCreateFromRouting = "kors.mes.of.create_from_routing"
	SubjectOFDispatchList      = "kors.mes.of.dispatch_list"
	SubjectOFSetPlanning       = "kors.mes.of.set_planning"

	// Async events — routings (BLOC 5)
	SubjectRoutingCreated = "kors.mes.routing.created"

	// Synchronous request-reply subjects — qualifications (AS9100D §7.2)
	SubjectQualificationCreate       = "kors.mes.qualification.create"
	SubjectQualificationGet          = "kors.mes.qualification.get"
	SubjectQualificationList         = "kors.mes.qualification.list"
	SubjectQualificationRenew        = "kors.mes.qualification.renew"
	SubjectQualificationRevoke       = "kors.mes.qualification.revoke"
	SubjectQualificationListActive   = "kors.mes.qualification.list_active_skills"
	SubjectQualificationListExpiring = "kors.mes.qualification.list_expiring"

	// Async events — qualifications (AS9100D §7.2)
	SubjectQualificationCreated       = "kors.mes.qualification.created"
	SubjectQualificationRenewed       = "kors.mes.qualification.renewed"
	SubjectQualificationRevoked       = "kors.mes.qualification.revoked"
	SubjectQualificationExpired       = "kors.mes.qualification.expired"
	SubjectQualificationExpiringAlert = "kors.mes.qualification.expiring_alert"

	// Synchronous request-reply subjects — workstations (BLOC 6)
	SubjectWorkstationCreate       = "kors.mes.workstation.create"
	SubjectWorkstationGet          = "kors.mes.workstation.get"
	SubjectWorkstationList         = "kors.mes.workstation.list"
	SubjectWorkstationUpdateStatus = "kors.mes.workstation.update_status"

	// Async events — workstations (BLOC 6)
	SubjectWorkstationCreated       = "kors.mes.workstation.created"
	SubjectWorkstationStatusChanged = "kors.mes.workstation.status_changed"

	// Synchronous request-reply subjects — time tracking & OEE (BLOC 5)
	SubjectTimeLogRecord      = "kors.mes.time_log.record"
	SubjectDowntimeStart      = "kors.mes.downtime.start"
	SubjectDowntimeEnd        = "kors.mes.downtime.end"
	SubjectWorkstationOEEGet  = "kors.mes.workstation.oee.get"

	// Async events — time tracking & OEE (BLOC 5)
	SubjectTimeLogRecorded    = "kors.mes.time_log.recorded"
	SubjectDowntimeStarted    = "kors.mes.downtime.started"
	SubjectDowntimeEnded      = "kors.mes.downtime.ended"

	// Synchronous request-reply subjects — tools & gauges (BLOC 8)
	SubjectToolCreate            = "kors.mes.tool.create"
	SubjectToolGet               = "kors.mes.tool.get"
	SubjectToolList              = "kors.mes.tool.list"
	SubjectToolCalibrate         = "kors.mes.tool.calibrate"
	SubjectToolAssignToOperation = "kors.mes.tool.assign_to_operation"
	SubjectOperationToolsList    = "kors.mes.operation.tools.list"

	// Async events — tools & gauges (BLOC 8)
	SubjectToolCreated           = "kors.mes.tool.created"
	SubjectToolCalibrationUpdated = "kors.mes.tool.calibration_updated"
	SubjectToolUsageRecorded     = "kors.mes.tool.usage_recorded"

	// Synchronous request-reply subjects — materials & WIP (BLOC 9)
	SubjectMaterialConsume    = "kors.mes.material.consume"
	SubjectMaterialTOEStart   = "kors.mes.material.toe.start"
	SubjectMaterialTOEStop    = "kors.mes.material.toe.stop"
	SubjectEntityTransfer     = "kors.mes.entity.transfer"

	// Async events — materials & WIP (BLOC 9)
	SubjectMaterialConsumed    = "kors.mes.material.consumed"
	SubjectLocationTransferred = "kors.mes.location.transferred"

	// Synchronous request-reply subjects — inline quality (BLOC 10)
	SubjectMeasurementRecord          = "kors.mes.measurement.record"
	SubjectOperationCharacteristicsList = "kors.mes.operation.characteristics.list"
	SubjectMeasurementHistoryList     = "kors.mes.measurement.history.list"

	// Async events — inline quality (BLOC 10)
	SubjectMeasurementRecorded = "kors.mes.measurement.recorded"
	SubjectQualityAlertRaised  = "kors.mes.quality.alert"

	// Synchronous request-reply subjects — alerts (BLOC 11)
	SubjectAlertRaise       = "kors.mes.alert.raise"
	SubjectAlertAcknowledge = "kors.mes.alert.acknowledge"
	SubjectAlertResolve     = "kors.mes.alert.resolve"
	SubjectAlertListActive  = "kors.mes.alert.list_active"

	// Async events — alerts (BLOC 11)
	SubjectAlertRaised      = "kors.mes.alert.raised"
	SubjectAlertEscalated   = "kors.mes.alert.escalated"
	SubjectAlertResolved    = "kors.mes.alert.resolved"
	SubjectAlertEscalationRequested = "kors.mes.alert.escalation_requested"

	// Synchronous request-reply subjects — compliance & audit trail (§13)
	SubjectAuditQuery  = "kors.mes.audit.query"
	SubjectAsBuiltGet  = "kors.mes.compliance.as_built.get"

	// Synchronous request-reply subjects — supervision & dashboards (§16)
	SubjectDashboardSupervisorGet = "kors.mes.dashboard.supervisor.get"
	SubjectMetricsTRSByPeriod     = "kors.mes.metrics.trs.by_period"
	SubjectMetricsDowntimeCauses  = "kors.mes.metrics.downtime_causes"
	SubjectMetricsProductionProgress = "kors.mes.metrics.production_progress"

	// Queue group name — all MES instances subscribe with this group for load balancing
	QueueGroupMES = "mes"
)
