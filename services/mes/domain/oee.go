package domain

import (
	"time"
)

// OEEData represents the calculated components of Overall Equipment Effectiveness.
type OEEData struct {
	Availability         float64
	Performance          float64
	Quality              float64
	TRS                  float64
	TotalDowntimeSeconds int
	TotalOperatingSeconds int
	TotalGoodQuantity    int
	TotalScrapQuantity   int
}

// OEECalculator provides methods to calculate TRS/OEE.
type OEECalculator struct {
	// Normally we would query the database here, but to keep domain pure,
	// we will define calculation functions that take raw inputs.
}

// CalculateOEE computes Availability, Performance, Quality, and overall TRS.
// plannedTime: the time the workstation was supposed to be running (e.g. shift duration).
// operatingTime: actual time spent processing (TimeLog duration where type is RUN).
// downtime: total duration of all downtime events.
// goodQty: total acceptable units produced.
// scrapQty: total rejected units produced.
// idealCycleTimeSeconds: theoretical time to produce one unit.
func CalculateOEE(
	plannedTime time.Duration,
	operatingTime time.Duration,
	downtime time.Duration,
	goodQty int,
	scrapQty int,
	idealCycleTimeSeconds float64,
) OEEData {
	data := OEEData{
		TotalDowntimeSeconds: int(downtime.Seconds()),
		TotalOperatingSeconds: int(operatingTime.Seconds()),
		TotalGoodQuantity:    goodQty,
		TotalScrapQuantity:   scrapQty,
	}

	totalQty := goodQty + scrapQty

	// Availability = (PlannedTime - Downtime) / PlannedTime
	// Alternatively, Availability = OperatingTime / PlannedTime (if OperatingTime = Planned - Down)
	if plannedTime > 0 {
		availableTime := plannedTime - downtime
		if availableTime < 0 {
			availableTime = 0
		}
		data.Availability = float64(availableTime.Seconds()) / float64(plannedTime.Seconds())
	}

	// Performance = (IdealCycleTime * TotalQty) / OperatingTime
	if operatingTime > 0 {
		theoreticalTime := idealCycleTimeSeconds * float64(totalQty)
		data.Performance = theoreticalTime / float64(operatingTime.Seconds())
		if data.Performance > 1.0 {
			// Cap at 100% in case of minor recording errors or running faster than nominal
			data.Performance = 1.0
		}
	}

	// Quality = GoodQty / TotalQty
	if totalQty > 0 {
		data.Quality = float64(goodQty) / float64(totalQty)
	} else {
		// If nothing produced, quality is functionally perfect (no scrap) but OEE will be 0 due to performance
		data.Quality = 1.0
	}

	data.TRS = data.Availability * data.Performance * data.Quality

	return data
}
