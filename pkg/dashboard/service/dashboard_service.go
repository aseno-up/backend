package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/aseno/backend/pkg/dashboard/model"
	"github.com/aseno/backend/pkg/dashboard/repository"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	profModel "github.com/aseno/backend/pkg/professional/model"
)

type DashboardService interface {
	GetMetrics(ctx context.Context, clinicID uuid.UUID, start, end time.Time) (*model.DashboardMetrics, error)
}

type dashboardService struct {
	repo       repository.DashboardRepository
	clinicRepo clinic_repository.ClinicRepository
}

func NewDashboardService(repo repository.DashboardRepository, clinicRepo clinic_repository.ClinicRepository) DashboardService {
	return &dashboardService{
		repo:       repo,
		clinicRepo: clinicRepo,
	}
}

func (s *dashboardService) GetMetrics(ctx context.Context, clinicID uuid.UUID, start, end time.Time) (*model.DashboardMetrics, error) {
	// 1. Get Appointment Stats
	stats, err := s.repo.GetAppointmentStats(ctx, clinicID, start, end)
	if err != nil {
		return nil, err
	}

	// 2. Get Previous Period Stats (for comparison)
	duration := end.Sub(start)
	prevStart := start.Add(-duration)
	prevEnd := start
	prevStats, err := s.repo.GetAppointmentStats(ctx, clinicID, prevStart, prevEnd)
	if err != nil {
		return nil, err
	}

	// 3. Get Top Absentees
	absentees, err := s.repo.GetTopAbsentees(ctx, clinicID, start, end, 3)
	if err != nil {
		return nil, err
	}

	// 4. Get Future Sessions
	futureSessions, err := s.repo.GetFutureSessionsCount(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	// 5. Calculate Occupancy & Open Slots
	profs, err := s.repo.GetProfessionals(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	totalCapacityMins := 0
	for _, p := range profs {
		var wh profModel.WorkingHours
		if err := json.Unmarshal(p.WorkingHours, &wh); err == nil {
			totalCapacityMins += s.calculateCapacity(wh, start, end)
		}
	}

	// Estimate 50 mins per appointment for MVP
	occupiedMins := stats.TotalCount * 50
	
	occupancy := 0.0
	openSlots := 0
	potentialRevenue := 0.0
	
	// Average price
	avgPrice := 0.0
	completedAndConfirmed := stats.CompletedCount + stats.ConfirmedCount
	if completedAndConfirmed > 0 {
		avgPrice = stats.TotalRevenue / float64(completedAndConfirmed)
	} else if stats.TotalCount > 0 {
		// Fallback to average of everything if no confirmed/completed
		avgPrice = (stats.TotalRevenue + stats.LostRevenue) / float64(stats.TotalCount)
	}
	if avgPrice == 0 {
		avgPrice = 100.0 // Fallback
	}

	if totalCapacityMins > 0 {
		occupancy = (float64(occupiedMins) / float64(totalCapacityMins)) * 100
		if occupancy > 100 {
			occupancy = 100
		}
		
		remainingMins := totalCapacityMins - occupiedMins
		if remainingMins < 0 {
			remainingMins = 0
		}
		openSlots = remainingMins / 50 // Assuming 50 min slots
		
		potentialRevenue = float64(openSlots) * avgPrice
	}

	// Attendance Rates
	attendanceRate := 0.0
	attended := stats.CompletedCount
	totalScheduled := stats.CompletedCount + stats.NoShowCount
	if totalScheduled > 0 {
		attendanceRate = (float64(attended) / float64(totalScheduled)) * 100
	}

	prevAttendanceRate := 0.0
	prevAttended := prevStats.CompletedCount
	prevTotalScheduled := prevStats.CompletedCount + prevStats.NoShowCount
	if prevTotalScheduled > 0 {
		prevAttendanceRate = (float64(prevAttended) / float64(prevTotalScheduled)) * 100
	}

	// Insights & Automation Impact
	// Estimate: 15% of auto-confirmed would have been no-shows without it
	avoidedNoShows := int(float64(stats.AutoConfirmed) * 0.15)
	preservedRevenue := float64(avoidedNoShows) * avgPrice

	var insights []string
	if attendanceRate < 80 {
		insights = append(insights, "Taxa de comparecimento abaixo de 80%. Reforce os lembretes.")
	}
	if stats.NoShowCount > 3 {
		insights = append(insights, fmt.Sprintf("%d faltas neste período. Verifique os pacientes recorrentes.", stats.NoShowCount))
	}
	if occupancy < 50 {
		insights = append(insights, "Ocupação da agenda abaixo de 50%. Considere ações de marketing.")
	}
	if len(absentees) > 0 {
		insights = append(insights, fmt.Sprintf("Paciente %s teve %d faltas.", absentees[0].Name, absentees[0].AbsentCount))
	}

	metrics := &model.DashboardMetrics{
		EstimatedRevenue:       stats.TotalRevenue,
		CompletedConsultations: stats.CompletedCount,
		ConfirmedConsultations: stats.ConfirmedCount,
		NoShows:                stats.NoShowCount,
		AttendanceRate:         attendanceRate,
		PreviousAttendanceRate: prevAttendanceRate,
		EstimatedLostRevenue:   stats.LostRevenue,
		AgendaOccupancy:        occupancy,
		OpenSlots:              openSlots,
		FutureSessions:         int(futureSessions),
		PotentialRevenue:       potentialRevenue,
		AvoidedNoShows:         avoidedNoShows,
		AutoConfirmed:          stats.AutoConfirmed,
		PreservedRevenue:       preservedRevenue,
		TopAbsentees:           absentees,
		Insights:               insights,
	}

	// Check plan and mask financial data if Basic
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err == nil && clinic.Plan == clinic_model.PlanTypeBasic {
		metrics.EstimatedRevenue = 0
		metrics.EstimatedLostRevenue = 0
		metrics.PotentialRevenue = 0
		metrics.PreservedRevenue = 0
		metrics.Insights = []string{"Faça upgrade para o plano Profissional para ver insights e dados financeiros."}
		// We keep operational stats (counts, occupancy) visible
	}

	return metrics, nil
}

func (s *dashboardService) calculateCapacity(wh profModel.WorkingHours, start, end time.Time) int {
	totalMins := 0
	current := start
	
	// Avoid infinite loop if start > end
	if start.After(end) {
		return 0
	}

	for current.Before(end) {
		weekday := current.Weekday()
		var slots []profModel.TimeSlot
		switch weekday {
		case time.Monday:
			slots = wh.Monday
		case time.Tuesday:
			slots = wh.Tuesday
		case time.Wednesday:
			slots = wh.Wednesday
		case time.Thursday:
			slots = wh.Thursday
		case time.Friday:
			slots = wh.Friday
		case time.Saturday:
			slots = wh.Saturday
		case time.Sunday:
			slots = wh.Sunday
		}

		for _, slot := range slots {
			h1, m1 := s.parseTime(slot.Start)
			h2, m2 := s.parseTime(slot.End)
			startMins := h1*60 + m1
			endMins := h2*60 + m2
			if endMins > startMins {
				totalMins += (endMins - startMins)
			}
		}
		current = current.AddDate(0, 0, 1)
	}
	return totalMins
}

func (s *dashboardService) parseTime(t string) (int, int) {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h, m
}
