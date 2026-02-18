package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/dashboard/model"
	"github.com/aseno/backend/pkg/dashboard/repository"
	profModel "github.com/aseno/backend/pkg/professional/model"
	"github.com/google/uuid"
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

	// Generate Smart Insights from time patterns
	smartInsights := s.generateSmartInsights(ctx, clinicID, start, end)

	// Calculate annual savings projection
	// Based on preserved revenue extrapolated to 365 days
	periodDays := end.Sub(start).Hours() / 24
	annualSavingsProjection := 0.0
	if periodDays > 0 {
		dailySavings := preservedRevenue / periodDays
		annualSavingsProjection = dailySavings * 365
	}

	metrics := &model.DashboardMetrics{
		EstimatedRevenue:        stats.TotalRevenue,
		CompletedConsultations:  stats.CompletedCount,
		ConfirmedConsultations:  stats.ConfirmedCount,
		NoShows:                 stats.NoShowCount,
		AttendanceRate:          attendanceRate,
		PreviousAttendanceRate:  prevAttendanceRate,
		EstimatedLostRevenue:    stats.LostRevenue,
		AgendaOccupancy:         occupancy,
		OpenSlots:               openSlots,
		FutureSessions:          int(futureSessions),
		PotentialRevenue:        potentialRevenue,
		AvoidedNoShows:          avoidedNoShows,
		AutoConfirmed:           stats.AutoConfirmed,
		PreservedRevenue:        preservedRevenue,
		AnnualSavingsProjection: annualSavingsProjection,
		TopAbsentees:            absentees,
		Insights:                insights,
		SmartInsights:           smartInsights,
	}

	// Check plan and mask financial data if Basic
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err == nil && clinic.Plan == clinic_model.PlanTypeBasic {
		metrics.EstimatedRevenue = 0
		metrics.EstimatedLostRevenue = 0
		metrics.PotentialRevenue = 0
		metrics.PreservedRevenue = 0
		metrics.AnnualSavingsProjection = 0
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

// generateSmartInsights creates intelligent insights based on time patterns
func (s *dashboardService) generateSmartInsights(ctx context.Context, clinicID uuid.UUID, start, end time.Time) []model.SmartInsight {
	var insights []model.SmartInsight

	// 1. Time period patterns (morning/afternoon/evening)
	timePeriods, err := s.repo.GetNoShowsByTimePeriod(ctx, clinicID, start, end)
	if err == nil && len(timePeriods) > 0 {
		// Find worst time period
		worst := timePeriods[0]
		if worst.NoShowRate >= 20 && worst.TotalCount >= 5 {
			periodName := s.translatePeriod(worst.Period)
			insights = append(insights, model.SmartInsight{
				Type:        "worst_time",
				Title:       fmt.Sprintf("Mais faltas %s", periodName),
				Description: fmt.Sprintf("%.0f%% das consultas %s resultam em faltas (%d de %d)", worst.NoShowRate, periodName, worst.NoShowCount, worst.TotalCount),
				Severity:    s.getSeverity(worst.NoShowRate),
				ActionHint:  fmt.Sprintf("Considere reforcar lembretes para consultas %s", periodName),
			})
		}

		// Find best time period (lowest no-show rate)
		if len(timePeriods) > 1 {
			best := timePeriods[len(timePeriods)-1]
			if best.NoShowRate < 15 && best.TotalCount >= 5 {
				periodName := s.translatePeriod(best.Period)
				insights = append(insights, model.SmartInsight{
					Type:        "best_time",
					Title:       fmt.Sprintf("Melhor horario: %s", periodName),
					Description: fmt.Sprintf("Consultas %s tem apenas %.0f%% de faltas", periodName, best.NoShowRate),
					Severity:    "info",
					ActionHint:  fmt.Sprintf("Priorize agendamentos %s para pacientes com historico de faltas", periodName),
				})
			}
		}
	}

	// 2. Day of week patterns
	dayPatterns, err := s.repo.GetNoShowsByDayOfWeek(ctx, clinicID, start, end)
	if err == nil && len(dayPatterns) > 0 {
		worst := dayPatterns[0]
		if worst.NoShowRate >= 25 && worst.TotalCount >= 3 {
			insights = append(insights, model.SmartInsight{
				Type:        "worst_day",
				Title:       fmt.Sprintf("%s tem mais faltas", worst.Period),
				Description: fmt.Sprintf("%.0f%% das consultas de %s resultam em faltas", worst.NoShowRate, worst.Period),
				Severity:    s.getSeverity(worst.NoShowRate),
				ActionHint:  fmt.Sprintf("Envie lembretes extras para consultas de %s", worst.Period),
			})
		}
	}

	// 3. Patient-specific time patterns
	patientPatterns, err := s.repo.GetPatientTimePatterns(ctx, clinicID, start, end, 3)
	if err == nil {
		for _, p := range patientPatterns {
			if p.NoShowRate >= 50 && p.NoShowCount >= 2 {
				periodName := s.translatePeriod(p.Period)
				insights = append(insights, model.SmartInsight{
					Type:        "patient_time",
					Title:       fmt.Sprintf("%s falta %s", s.firstName(p.PatientName), periodName),
					Description: fmt.Sprintf("%s tem %.0f%% de faltas em consultas %s (%d de %d)", s.firstName(p.PatientName), p.NoShowRate, periodName, p.NoShowCount, p.TotalCount),
					Severity:    "warning",
					ActionHint:  fmt.Sprintf("Evite agendar %s %s ou faca confirmacao no dia", s.firstName(p.PatientName), periodName),
				})
				// Limit to 2 patient insights
				if len(insights) >= 2 {
					break
				}
			}
		}
	}

	// 4. Professional-specific day patterns
	profPatterns, err := s.repo.GetProfessionalDayPatterns(ctx, clinicID, start, end, 5)
	if err == nil {
		for _, p := range profPatterns {
			if p.NoShowRate >= 30 && p.NoShowCount >= 2 {
				insights = append(insights, model.SmartInsight{
					Type:        "professional_day",
					Title:       fmt.Sprintf("%s: mais faltas %s", s.firstName(p.ProfessionalName), p.DayOfWeek),
					Description: fmt.Sprintf("Pacientes de %s faltam %.0f%% nas %ss (%d de %d)", s.firstName(p.ProfessionalName), p.NoShowRate, p.DayOfWeek, p.NoShowCount, p.TotalCount),
					Severity:    "warning",
					ActionHint:  "Analise se ha algo especifico neste dia que causa faltas",
				})
				// Limit to 1 professional insight
				break
			}
		}
	}

	return insights
}

// translatePeriod converts period code to Portuguese
func (s *dashboardService) translatePeriod(period string) string {
	switch period {
	case "morning":
		return "pela manha"
	case "afternoon":
		return "a tarde"
	case "evening":
		return "a noite"
	default:
		return period
	}
}

// getSeverity returns severity based on no-show rate
func (s *dashboardService) getSeverity(rate float64) string {
	if rate >= 40 {
		return "critical"
	}
	if rate >= 25 {
		return "warning"
	}
	return "info"
}

// firstName extracts first name from full name
func (s *dashboardService) firstName(fullName string) string {
	parts := []rune(fullName)
	for i, r := range parts {
		if r == ' ' {
			return string(parts[:i])
		}
	}
	return fullName
}
