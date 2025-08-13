package budget

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	
	"github.com/amerfu/pllm/internal/models"
)

var (
	ErrBudgetExceeded = errors.New("budget exceeded")
	ErrBudgetNotFound = errors.New("budget not found")
	ErrInvalidPeriod  = errors.New("invalid budget period")
)

type BudgetService struct {
	db       *gorm.DB
	logger   *zap.Logger
	mu       sync.RWMutex
	cache    map[uuid.UUID]*models.Budget
	stopCh   chan struct{}
	alertCh  chan *models.Budget
}

type BudgetConfig struct {
	DB             *gorm.DB
	Logger         *zap.Logger
	CheckInterval  time.Duration
	AlertWebhook   string
	AlertEmails    []string
}

type BudgetCreateRequest struct {
	Name      string                `json:"name"`
	Type      models.BudgetType     `json:"type"`
	Amount    float64               `json:"amount"`
	Period    models.BudgetPeriod   `json:"period"`
	UserID    *uuid.UUID            `json:"user_id,omitempty"`
	GroupID   *uuid.UUID            `json:"group_id,omitempty"`
	AlertAt   float64               `json:"alert_at"`
	Actions   []models.BudgetAction `json:"actions,omitempty"`
	StartsAt  *time.Time            `json:"starts_at,omitempty"`
	EndsAt    *time.Time            `json:"ends_at,omitempty"`
}

type BudgetUpdateRequest struct {
	Name     string                `json:"name,omitempty"`
	Amount   float64               `json:"amount,omitempty"`
	AlertAt  float64               `json:"alert_at,omitempty"`
	IsActive bool                  `json:"is_active"`
	Actions  []models.BudgetAction `json:"actions,omitempty"`
}

type BudgetUsage struct {
	BudgetID   uuid.UUID `json:"budget_id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Amount     float64   `json:"amount"`
	Spent      float64   `json:"spent"`
	Remaining  float64   `json:"remaining"`
	Percentage float64   `json:"percentage"`
	Period     string    `json:"period"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	IsActive   bool      `json:"is_active"`
	AlertSent  bool      `json:"alert_sent"`
}

func NewBudgetService(config *BudgetConfig) *BudgetService {
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Minute
	}

	service := &BudgetService{
		db:      config.DB,
		logger:  config.Logger,
		cache:   make(map[uuid.UUID]*models.Budget),
		stopCh:  make(chan struct{}),
		alertCh: make(chan *models.Budget, 100),
	}

	go service.startBudgetMonitor(config.CheckInterval)
	go service.startAlertProcessor()

	return service
}

func (s *BudgetService) CreateBudget(ctx context.Context, req *BudgetCreateRequest) (*models.Budget, error) {
	budget := &models.Budget{
		Name:     req.Name,
		Type:     req.Type,
		Amount:   req.Amount,
		Period:   req.Period,
		UserID:   req.UserID,
		GroupID:  req.GroupID,
		AlertAt:  req.AlertAt,
		Actions:  req.Actions,
		IsActive: true,
	}

	if req.AlertAt == 0 {
		budget.AlertAt = 80
	}

	now := time.Now()
	if req.StartsAt != nil {
		budget.StartsAt = *req.StartsAt
	} else {
		budget.StartsAt = now
	}

	if req.EndsAt != nil {
		budget.EndsAt = *req.EndsAt
	} else {
		switch req.Period {
		case models.BudgetPeriodDaily:
			budget.EndsAt = now.AddDate(0, 0, 1)
		case models.BudgetPeriodWeekly:
			budget.EndsAt = now.AddDate(0, 0, 7)
		case models.BudgetPeriodMonthly:
			budget.EndsAt = now.AddDate(0, 1, 0)
		case models.BudgetPeriodYearly:
			budget.EndsAt = now.AddDate(1, 0, 0)
		case models.BudgetPeriodCustom:
			if req.EndsAt == nil {
				return nil, ErrInvalidPeriod
			}
		}
	}

	if err := s.db.Create(budget).Error; err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[budget.ID] = budget
	s.mu.Unlock()

	return budget, nil
}

func (s *BudgetService) GetBudget(ctx context.Context, id uuid.UUID) (*models.Budget, error) {
	s.mu.RLock()
	if budget, ok := s.cache[id]; ok {
		s.mu.RUnlock()
		return budget, nil
	}
	s.mu.RUnlock()

	var budget models.Budget
	if err := s.db.Preload("User").Preload("Group").First(&budget, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBudgetNotFound
		}
		return nil, err
	}

	s.mu.Lock()
	s.cache[budget.ID] = &budget
	s.mu.Unlock()

	return &budget, nil
}

func (s *BudgetService) UpdateBudget(ctx context.Context, id uuid.UUID, req *BudgetUpdateRequest) (*models.Budget, error) {
	budget, err := s.GetBudget(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		budget.Name = req.Name
	}
	if req.Amount > 0 {
		budget.Amount = req.Amount
	}
	if req.AlertAt > 0 {
		budget.AlertAt = req.AlertAt
	}
	budget.IsActive = req.IsActive
	
	if len(req.Actions) > 0 {
		budget.Actions = req.Actions
	}

	if err := s.db.Save(budget).Error; err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[budget.ID] = budget
	s.mu.Unlock()

	return budget, nil
}

func (s *BudgetService) DeleteBudget(ctx context.Context, id uuid.UUID) error {
	if err := s.db.Delete(&models.Budget{}, "id = ?", id).Error; err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()

	return nil
}

func (s *BudgetService) ListBudgets(ctx context.Context, userID, groupID *uuid.UUID) ([]*models.Budget, error) {
	var budgets []*models.Budget
	query := s.db.Model(&models.Budget{})

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	}

	if err := query.Find(&budgets).Error; err != nil {
		return nil, err
	}

	return budgets, nil
}

func (s *BudgetService) GetBudgetUsage(ctx context.Context, id uuid.UUID) (*BudgetUsage, error) {
	budget, err := s.GetBudget(ctx, id)
	if err != nil {
		return nil, err
	}

	return &BudgetUsage{
		BudgetID:   budget.ID,
		Name:       budget.Name,
		Type:       string(budget.Type),
		Amount:     budget.Amount,
		Spent:      budget.Spent,
		Remaining:  budget.GetRemainingBudget(),
		Percentage: budget.GetUsagePercentage(),
		Period:     string(budget.Period),
		StartsAt:   budget.StartsAt,
		EndsAt:     budget.EndsAt,
		IsActive:   budget.IsActive,
		AlertSent:  budget.AlertSent,
	}, nil
}

func (s *BudgetService) RecordUsage(ctx context.Context, userID, groupID *uuid.UUID, amount float64) error {
	var budgets []*models.Budget
	query := s.db.Model(&models.Budget{}).Where("is_active = ?", true)

	if userID != nil {
		query = query.Where("user_id = ? OR type = ?", *userID, models.BudgetTypeGlobal)
	}
	if groupID != nil {
		query = query.Where("group_id = ? OR type = ?", *groupID, models.BudgetTypeGlobal)
	}

	if err := query.Find(&budgets).Error; err != nil {
		return err
	}

	for _, budget := range budgets {
		if budget.IsExpired() {
			if s.shouldResetBudget(budget) {
				budget.Reset()
			} else {
				budget.IsActive = false
			}
		}

		budget.Spent += amount

		if budget.IsExceeded() {
			s.executeActions(budget, 100)
			if !budget.IsExceeded() {
				return ErrBudgetExceeded
			}
		} else if budget.ShouldAlert() {
			s.alertCh <- budget
			budget.AlertSent = true
		}

		s.executeActions(budget, budget.GetUsagePercentage())

		if err := s.db.Save(budget).Error; err != nil {
			return err
		}

		s.mu.Lock()
		s.cache[budget.ID] = budget
		s.mu.Unlock()
	}

	return nil
}

func (s *BudgetService) ResetBudget(ctx context.Context, id uuid.UUID) error {
	budget, err := s.GetBudget(ctx, id)
	if err != nil {
		return err
	}

	budget.Reset()

	if err := s.db.Save(budget).Error; err != nil {
		return err
	}

	s.mu.Lock()
	s.cache[budget.ID] = budget
	s.mu.Unlock()

	return nil
}

func (s *BudgetService) startBudgetMonitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndResetBudgets()
		case <-s.stopCh:
			return
		}
	}
}

func (s *BudgetService) checkAndResetBudgets() {
	var budgets []*models.Budget
	s.db.Where("is_active = ?", true).Find(&budgets)

	for _, budget := range budgets {
		if budget.IsExpired() && s.shouldResetBudget(budget) {
			budget.Reset()
			s.db.Save(budget)
			
			s.mu.Lock()
			s.cache[budget.ID] = budget
			s.mu.Unlock()
		}
	}
}

func (s *BudgetService) shouldResetBudget(budget *models.Budget) bool {
	return budget.Period != models.BudgetPeriodCustom
}

func (s *BudgetService) executeActions(budget *models.Budget, percentage float64) {
	for i := range budget.Actions {
		action := &budget.Actions[i]
		if !action.Executed && percentage >= action.Threshold {
			s.executeAction(budget, action)
			action.Executed = true
			now := time.Now()
			action.ExecutedAt = &now
		}
	}
}

func (s *BudgetService) executeAction(budget *models.Budget, action *models.BudgetAction) {
	switch action.Action {
	case "alert":
		s.alertCh <- budget
	case "throttle":
		// Implement rate limiting by reducing available rate limits
		// This could be done by updating team/user rate limits in real-time
	case "block":
		budget.IsActive = false
	case "webhook":
		// Send webhook notification
		go s.sendWebhookNotification(budget, action)
	}
}

func (s *BudgetService) startAlertProcessor() {
	for {
		select {
		case budget := <-s.alertCh:
			s.sendAlert(budget)
		case <-s.stopCh:
			return
		}
	}
}

func (s *BudgetService) sendAlert(budget *models.Budget) {
	// Send budget alert notifications
	s.logger.Warn("Budget alert triggered",
		zap.String("budget_name", budget.Name),
		zap.Float64("usage_percentage", budget.GetUsagePercentage()),
		zap.Float64("amount", budget.Amount),
		zap.Float64("spent", budget.Spent))
	
	// TODO: Implement email notifications
	// TODO: Implement webhook notifications
	fmt.Printf("Budget alert: %s has used %.2f%% of budget\n", budget.Name, budget.GetUsagePercentage())
}

func (s *BudgetService) Stop() {
	close(s.stopCh)
}

// sendWebhookNotification sends a webhook notification for budget actions
func (s *BudgetService) sendWebhookNotification(budget *models.Budget, action *models.BudgetAction) {
	// TODO: Implement webhook sending logic
	s.logger.Info("Webhook notification triggered",
		zap.String("budget_name", budget.Name),
		zap.String("action", action.Action),
		zap.Float64("threshold", action.Threshold),
		zap.Float64("usage_percentage", budget.GetUsagePercentage()))
}