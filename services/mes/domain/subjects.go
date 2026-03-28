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

	// Queue group name — all MES instances subscribe with this group for load balancing
	QueueGroupMES = "mes"
)
