package domain

// NATS subject constants for the QMS domain.
const (
	// Synchronous request-reply subjects — non-conformities
	SubjectNCGet                = "kors.qms.nc.get"
	SubjectNCList               = "kors.qms.nc.list"
	SubjectNCAnalyse            = "kors.qms.nc.analyse"
	SubjectNCProposeDisposition = "kors.qms.nc.propose_disposition"
	SubjectNCClose              = "kors.qms.nc.close"

	// Synchronous request-reply subjects — CAPAs
	SubjectCAPACreate   = "kors.qms.capa.create"
	SubjectCAPAGet      = "kors.qms.capa.get"
	SubjectCAPAList     = "kors.qms.capa.list"
	SubjectCAPAStart    = "kors.qms.capa.start"
	SubjectCAPAComplete = "kors.qms.capa.complete"

	// Async events published to NATS JetStream
	SubjectNCOpened     = "kors.qms.nc.opened"
	SubjectNCClosed     = "kors.qms.nc.closed"
	SubjectCAPACreated  = "kors.qms.capa.created"

	// Consumed from MES
	SubjectMESNCDeclared = "kors.mes.nc.declared"

	// Queue group — all QMS instances subscribe with this group for load balancing
	QueueGroupQMS = "qms"
)
