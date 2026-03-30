package domain

import (
	"time"
)

// SupervisorSnapshot contains a real-time view of the shop floor.
type SupervisorSnapshot struct {
	ActiveOrders  []*Order
	Workstations  []*WorkstationSnapshot
	ActiveAlerts  []*Alert
}

// WorkstationSnapshot contains the current status and OEE of a workstation.
type WorkstationSnapshot struct {
	WorkstationID   string
	WorkstationName string
	Status          WorkstationStatus
	CurrentOFID     string
	CurrentOFRef    string
	OEE             OEEData
}

// TRSFilter defines criteria for TRS reports.
type TRSFilter struct {
	WorkstationID string
	From          time.Time
	To            time.Time
	Granularity   TRSPeriodGranularity
}

// TRSPeriodGranularity defines the bucket size for TRS aggregation.
type TRSPeriodGranularity string

const (
	TRSPeriodDay   TRSPeriodGranularity = "DAY"
	TRSPeriodWeek  TRSPeriodGranularity = "WEEK"
	TRSPeriodMonth TRSPeriodGranularity = "MONTH"
)

// TRSDataPoint represents TRS components for a specific period.
type TRSDataPoint struct {
	Period       string
	TRS          float64
	Availability float64
	Performance  float64
	Quality      float64
}

// DowntimeCause represents an aggregated reason for machine downtime.
type DowntimeCause struct {
	Reason              string
	TotalDurationSeconds float64
	OccurrenceCount      int
}

// ProgressLine represents the production status of a manufacturing order.
type ProgressLine struct {
	OFID                 string
	OFReference          string
	ProductID            string
	PlannedQuantity      int
	GoodQuantity         int
	ScrapQuantity        int
	CompletionPercentage float64
}
