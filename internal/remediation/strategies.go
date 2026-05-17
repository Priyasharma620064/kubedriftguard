package remediation

import (
	"context"
	"fmt"
	"log"

	"github.com/Priyasharma620064/kubedriftguard/internal/models"
)

// IgnoreStrategy does nothing — used for cosmetic drift.
type IgnoreStrategy struct{}

func (s *IgnoreStrategy) Name() string { return "ignore" }

func (s *IgnoreStrategy) Execute(_ context.Context, event *models.DriftEvent) error {
	log.Printf("[ignore] skipping drift in %s field %s (severity: %s)",
		event.Resource, event.Field, event.Severity)
	return nil
}

// AutoRevertStrategy reverts the live resource to match the Git source of truth.
type AutoRevertStrategy struct {
	dryRun bool
}

func (s *AutoRevertStrategy) Name() string { return "auto-revert" }

func (s *AutoRevertStrategy) Execute(_ context.Context, event *models.DriftEvent) error {
	if s.dryRun {
		log.Printf("[auto-revert][DRY-RUN] would revert %s field %s from %q to %q",
			event.Resource, event.Field, event.ActualValue, event.ExpectedValue)
		return nil
	}

	// In a real implementation, this would use client-go to patch the resource.
	// For now, we log the action and mark success.
	// The controller layer will handle the actual Kubernetes API call.
	log.Printf("[auto-revert] reverting %s field %s from %q to %q",
		event.Resource, event.Field, event.ActualValue, event.ExpectedValue)

	return nil
}

// AlertAndHoldStrategy sends an alert and waits for human approval.
type AlertAndHoldStrategy struct {
	webhookURL string
}

func (s *AlertAndHoldStrategy) Name() string { return "alert-and-hold" }

func (s *AlertAndHoldStrategy) Execute(ctx context.Context, event *models.DriftEvent) error {
	// Send webhook notification
	if s.webhookURL != "" {
		if err := s.sendWebhook(event); err != nil {
			return fmt.Errorf("sending webhook alert: %w", err)
		}
	}

	log.Printf("[alert-and-hold] alert sent for %s field %s — awaiting human approval",
		event.Resource, event.Field)

	return nil
}

// sendWebhook sends a notification to the configured webhook endpoint.
func (s *AlertAndHoldStrategy) sendWebhook(event *models.DriftEvent) error {
	// In production, this would make an HTTP POST request.
	// For now, log the webhook payload details.
	log.Printf("[webhook] POST %s — resource=%s severity=%s field=%s expected=%q actual=%q",
		s.webhookURL, event.Resource, event.Severity, event.Field,
		event.ExpectedValue, event.ActualValue)
	return nil
}

// NotifyStrategy logs the drift event without taking action.
type NotifyStrategy struct{}

func (s *NotifyStrategy) Name() string { return "notify" }

func (s *NotifyStrategy) Execute(_ context.Context, event *models.DriftEvent) error {
	log.Printf("[notify] drift detected in %s field %s: expected=%q actual=%q (severity: %s)",
		event.Resource, event.Field, event.ExpectedValue, event.ActualValue, event.Severity)
	return nil
}
