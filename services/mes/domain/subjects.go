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
	SubjectOFCancelled        = "kors.mes.of.cancelled"
	SubjectOperationStarted   = "kors.mes.operation.started"
	SubjectOperationCompleted = "kors.mes.operation.completed"

	// Synchronous request-reply subjects
	SubjectOFCreate          = "kors.mes.of.create"
	SubjectOFGet             = "kors.mes.of.get"
	SubjectOFList            = "kors.mes.of.list"
	SubjectOperationStart    = "kors.mes.operation.start"
	SubjectOperationComplete = "kors.mes.operation.complete"

	// Queue group name — all MES instances subscribe with this group for load balancing
	QueueGroupMES = "mes"
)
