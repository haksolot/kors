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

	// Queue group name — all MES instances subscribe with this group for load balancing
	QueueGroupMES = "mes"
)
