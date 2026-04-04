package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/haksolot/kors/services/mes/domain"
)

// GetSupervisorSnapshot returns a real-time view of the shop floor.
func (r *PostgresRepo) GetSupervisorSnapshot(ctx context.Context) (*domain.SupervisorSnapshot, error) {
	snapshot := &domain.SupervisorSnapshot{}

	// 1. Active Orders
	orders, err := r.DispatchList(ctx, 100)
	if err != nil {
		return nil, fmt.Errorf("snapshot active orders: %w", err)
	}
	snapshot.ActiveOrders = orders

	// 2. Active Alerts
	alerts, err := r.ListActiveAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot active alerts: %w", err)
	}
	snapshot.ActiveAlerts = alerts

	// 3. Workstations Snapshot
	wsList, err := r.ListWorkstations(ctx, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshot list workstations: %w", err)
	}

	now := time.Now().UTC()
	eightHoursAgo := now.Add(-8 * time.Hour)

	for _, ws := range wsList {
		wsSnap := &domain.WorkstationSnapshot{
			WorkstationID:   ws.ID,
			WorkstationName: ws.Name,
			Status:          ws.Status,
		}

		// Find current/last OF reference
		var ofRef, ofID string
		err := r.db.QueryRow(ctx,
			`SELECT mo.id, mo.reference
			 FROM time_logs tl
			 JOIN operations op ON tl.operation_id = op.id
			 JOIN manufacturing_orders mo ON op.of_id = mo.id
			 WHERE tl.workstation_id = $1
			 ORDER BY tl.end_time DESC
			 LIMIT 1`, ws.ID,
		).Scan(&ofID, &ofRef)
		if err == nil {
			wsSnap.CurrentOFID = ofID
			wsSnap.CurrentOFRef = ofRef
		}

		// Calculate 8h OEE
		oee, err := r.calculateWorkstationOEE(ctx, ws, eightHoursAgo, now)
		if err == nil {
			wsSnap.OEE = oee
		}

		snapshot.Workstations = append(snapshot.Workstations, wsSnap)
	}

	return snapshot, nil
}

func (r *PostgresRepo) calculateWorkstationOEE(ctx context.Context, ws *domain.Workstation, from, to time.Time) (domain.OEEData, error) {
	// Query logs
	logs, err := r.ListTimeLogsByWorkstation(ctx, ws.ID, from, to)
	if err != nil {
		return domain.OEEData{}, err
	}

	// Query downtimes
	downtimes, err := r.ListDowntimesByWorkstation(ctx, ws.ID, from, to)
	if err != nil {
		return domain.OEEData{}, err
	}

	var operatingTime time.Duration
	var downtime time.Duration
	var goodQty, scrapQty int

	for _, l := range logs {
		if l.LogType == domain.TimeLogTypeRun {
			operatingTime += l.Duration()
		}
		goodQty += l.GoodQuantity
		scrapQty += l.ScrapQuantity
	}

	for _, dt := range downtimes {
		dtStart := dt.StartTime
		if dtStart.Before(from) {
			dtStart = from
		}
		dtEnd := to
		if dt.EndTime != nil && dt.EndTime.Before(to) {
			dtEnd = *dt.EndTime
		}
		if dtEnd.After(dtStart) {
			downtime += dtEnd.Sub(dtStart)
		}
	}

	plannedTime := to.Sub(from)
	var idealCycleTime float64 = 0
	if ws.NominalRate > 0 {
		idealCycleTime = 3600.0 / ws.NominalRate
	}

	return domain.CalculateOEE(plannedTime, operatingTime, downtime, goodQty, scrapQty, idealCycleTime), nil
}

// GetTRSByPeriod returns TRS components aggregated by period.
func (r *PostgresRepo) GetTRSByPeriod(ctx context.Context, filter domain.TRSFilter) ([]*domain.TRSDataPoint, error) {
	// This is a complex query that needs to aggregate time_logs and downtime_events by period.
	// For the sake of the MVP, we will iterate through periods in Go or use a set of queries.
	// Let's use a simpler approach: group by period in SQL for raw sums, then compute OEE in Go.

	trunc := "day"
	switch filter.Granularity {
	case domain.TRSPeriodWeek:
		trunc = "week"
	case domain.TRSPeriodMonth:
		trunc = "month"
	}

	// 1. Get periods
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT date_trunc('%s', series)::text as period
		 FROM generate_series($1::timestamp, $2::timestamp, '1 %s'::interval) as series`, trunc, trunc),
		filter.From, filter.To,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		periods = append(periods, p)
	}

	// For each period, calculate OEE
	var results []*domain.TRSDataPoint
	for i, p := range periods {
		pStart, _ := time.Parse("2006-01-02 15:04:05", p)
		pEnd := pStart.AddDate(0, 0, 1) // default Day
		if filter.Granularity == domain.TRSPeriodWeek {
			pEnd = pStart.AddDate(0, 0, 7)
		} else if filter.Granularity == domain.TRSPeriodMonth {
			pEnd = pStart.AddDate(0, 1, 0)
		}

		// Clip to filter.To
		if pEnd.After(filter.To) {
			pEnd = filter.To
		}
		if pStart.Before(filter.From) && i == 0 {
			pStart = filter.From
		}

		// Aggregate for all workstations if filter.WorkstationID is empty
		// We'll need a global nominal rate or average it. This is tricky.
		// For simplicity, if global, we sum all operating times and good quantities.
		
		var totalOEE domain.OEEData
		if filter.WorkstationID != "" {
			ws, err := r.FindWorkstationByID(ctx, filter.WorkstationID)
			if err == nil {
				totalOEE, _ = r.calculateWorkstationOEE(ctx, ws, pStart, pEnd)
			}
		} else {
			// Global aggregation: sum of OEE weighted by workstation? 
			// Or just sum of all times. Let's do sum of all workstations' OEE / count.
			wsList, _ := r.ListWorkstations(ctx, 1000, 0)
			var sumTRS, sumAvail, sumPerf, sumQual float64
			var count int
			for _, ws := range wsList {
				oee, err := r.calculateWorkstationOEE(ctx, ws, pStart, pEnd)
				if err == nil {
					sumTRS += oee.TRS
					sumAvail += oee.Availability
					sumPerf += oee.Performance
					sumQual += oee.Quality
					count++
				}
			}
			if count > 0 {
				totalOEE = domain.OEEData{
					TRS:          sumTRS / float64(count),
					Availability: sumAvail / float64(count),
					Performance:  sumPerf / float64(count),
					Quality:      sumQual / float64(count),
				}
			}
		}

		results = append(results, &domain.TRSDataPoint{
			Period:       p,
			TRS:          totalOEE.TRS,
			Availability: totalOEE.Availability,
			Performance:  totalOEE.Performance,
			Quality:      totalOEE.Quality,
		})
	}

	return results, nil
}

// GetDowntimeCauses returns aggregated reasons for downtime.
func (r *PostgresRepo) GetDowntimeCauses(ctx context.Context, from, to time.Time) ([]*domain.DowntimeCause, error) {
	rows, err := r.db.Query(ctx,
		`SELECT category, 
		        SUM(EXTRACT(EPOCH FROM (COALESCE(end_time, $2) - GREATEST(start_time, $1)))) as duration,
		        COUNT(*) as count
		 FROM downtime_events
		 WHERE start_time < $2 AND (end_time IS NULL OR end_time > $1)
		 GROUP BY category
		 ORDER BY duration DESC`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var causes []*domain.DowntimeCause
	for rows.Next() {
		var c domain.DowntimeCause
		if err := rows.Scan(&c.Reason, &c.TotalDurationSeconds, &c.OccurrenceCount); err != nil {
			return nil, err
		}
		causes = append(causes, &c)
	}
	return causes, nil
}

// GetProductionProgress returns the status of manufacturing orders.
func (r *PostgresRepo) GetProductionProgress(ctx context.Context, from, to time.Time) ([]*domain.ProgressLine, error) {
	rows, err := r.db.Query(ctx,
		`SELECT mo.id, mo.reference, mo.product_id, mo.quantity,
		        COALESCE(SUM(tl.good_qty), 0) as good,
		        COALESCE(SUM(tl.scrap_qty), 0) as scrap
		 FROM manufacturing_orders mo
		 LEFT JOIN operations op ON op.of_id = mo.id
		 LEFT JOIN time_logs tl ON tl.operation_id = op.id
		 WHERE mo.created_at >= $1 AND mo.created_at <= $2
		 GROUP BY mo.id
		 ORDER BY mo.created_at DESC`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []*domain.ProgressLine
	for rows.Next() {
		var l domain.ProgressLine
		if err := rows.Scan(&l.OFID, &l.OFReference, &l.ProductID, &l.PlannedQuantity, &l.GoodQuantity, &l.ScrapQuantity); err != nil {
			return nil, err
		}
		if l.PlannedQuantity > 0 {
			l.CompletionPercentage = float64(l.GoodQuantity) / float64(l.PlannedQuantity)
		}
		lines = append(lines, &l)
	}
	return lines, nil
}
