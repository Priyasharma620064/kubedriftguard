// Package remediation implements the self-healing engine that applies
// corrective actions when configuration drift is detected.
package remediation

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Priyasharma620064/kubedriftguard/internal/agents"
	"github.com/Priyasharma620064/kubedriftguard/internal/models"
)

// Engine is the self-healing remediation engine.
type Engine struct {
	config     *agents.AgentsConfig
	dryRun     bool
	maxRetries int
	strategies map[models.RemediationAction]Strategy
}

// Strategy defines a pluggable remediation behavior.
type Strategy interface {
	// Execute applies the remediation for a given drift event.
	Execute(ctx context.Context, event *models.DriftEvent) error

	// Name returns the human-readable name of this strategy.
	Name() string
}

// Result captures the outcome of a remediation attempt.
type Result struct {
	Event     models.DriftEvent
	Action    models.RemediationAction
	Success   bool
	Message   string
	Timestamp time.Time
	Attempts  int
}

// NewEngine creates a new remediation engine.
func NewEngine(config *agents.AgentsConfig, dryRun bool, maxRetries int) *Engine {
	e := &Engine{
		config:     config,
		dryRun:     dryRun,
		maxRetries: maxRetries,
		strategies: make(map[models.RemediationAction]Strategy),
	}

	// Register built-in strategies
	e.strategies[models.ActionIgnore] = &IgnoreStrategy{}
	e.strategies[models.ActionAutoRevert] = &AutoRevertStrategy{dryRun: dryRun}
	e.strategies[models.ActionAlertAndHold] = &AlertAndHoldStrategy{webhookURL: config.WebhookURL}
	e.strategies[models.ActionNotify] = &NotifyStrategy{}

	return e
}

// Remediate processes a list of drift events and applies the appropriate
// remediation action based on AGENTS.md configuration.
func (e *Engine) Remediate(ctx context.Context, events []models.DriftEvent) []Result {
	results := make([]Result, 0, len(events))

	for i := range events {
		event := &events[i]

		// Determine action from AGENTS.md config
		action := e.config.GetAction(event.Severity)
		event.Action = action

		// Check namespace protection
		if e.config.IsNamespaceProtected(event.Resource.Namespace) {
			results = append(results, Result{
				Event:     *event,
				Action:    action,
				Success:   false,
				Message:   fmt.Sprintf("namespace %q is protected — skipping remediation", event.Resource.Namespace),
				Timestamp: time.Now(),
			})
			continue
		}

		// Execute the strategy
		result := e.executeWithRetry(ctx, event, action)
		results = append(results, result)
	}

	return results
}

// executeWithRetry runs the remediation strategy with configurable retries.
func (e *Engine) executeWithRetry(ctx context.Context, event *models.DriftEvent, action models.RemediationAction) Result {
	strategy, ok := e.strategies[action]
	if !ok {
		return Result{
			Event:     *event,
			Action:    action,
			Success:   false,
			Message:   fmt.Sprintf("unknown remediation action: %s", action),
			Timestamp: time.Now(),
		}
	}

	var lastErr error
	for attempt := 1; attempt <= e.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return Result{
				Event:     *event,
				Action:    action,
				Success:   false,
				Message:   fmt.Sprintf("context cancelled: %v", err),
				Timestamp: time.Now(),
				Attempts:  attempt - 1,
			}
		}

		log.Printf("[remediation] %s: applying %s strategy for %s (attempt %d/%d)",
			event.Resource, strategy.Name(), event.Field, attempt, e.maxRetries)

		lastErr = strategy.Execute(ctx, event)
		if lastErr == nil {
			event.ResolvedAt = time.Now()
			event.ActionResult = fmt.Sprintf("resolved by %s strategy", strategy.Name())
			return Result{
				Event:     *event,
				Action:    action,
				Success:   true,
				Message:   fmt.Sprintf("successfully applied %s", strategy.Name()),
				Timestamp: time.Now(),
				Attempts:  attempt,
			}
		}

		log.Printf("[remediation] %s: attempt %d failed: %v", event.Resource, attempt, lastErr)

		// Backoff before retry
		if attempt < e.maxRetries {
			select {
			case <-ctx.Done():
				break
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
	}

	return Result{
		Event:     *event,
		Action:    action,
		Success:   false,
		Message:   fmt.Sprintf("failed after %d attempts: %v", e.maxRetries, lastErr),
		Timestamp: time.Now(),
		Attempts:  e.maxRetries,
	}
}
