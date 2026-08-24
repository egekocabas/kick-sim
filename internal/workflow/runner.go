package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/scenario"
)

type Options struct {
	Destination      string
	DestinationURL   string
	SubscriptionID   string
	Payload          map[string]any
	Start            time.Time
	SkipWait         bool
	Deliveries       int
	SameMessageID    bool
	ExpectedStatuses []int
}

type WorkflowResult struct {
	ScenarioID string     `json:"scenarioId"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Passed     bool       `json:"passed"`
	StartedAt  time.Time  `json:"startedAt"`
	EndedAt    time.Time  `json:"endedAt"`
	Deliveries []Delivery `json:"deliveries"`
	Error      string     `json:"error,omitempty"`
}

type Delivery struct {
	Step      int           `json:"step"`
	Iteration int           `json:"iteration"`
	Result    app.RunResult `json:"result"`
}

func Run(ctx context.Context, service *app.Service, entry scenario.Entry, options Options) (WorkflowResult, error) {
	start := options.Start.UTC()
	if start.IsZero() {
		start = service.Now().UTC()
	}
	result := WorkflowResult{ScenarioID: entry.ID, Name: entry.Scenario.Name, Kind: entry.Scenario.Kind(), StartedAt: start, Passed: true}
	var err error
	if entry.Scenario.Kind() == "timeline" {
		err = runTimeline(ctx, service, entry, options, start, &result)
	} else {
		err = runSingle(ctx, service, entry, options, start, &result)
	}
	result.EndedAt = service.Now().UTC()
	if err != nil {
		result.Passed = false
		result.Error = err.Error()
	}
	return result, err
}

func runSingle(ctx context.Context, service *app.Service, entry scenario.Entry, options Options, logicalTime time.Time, report *WorkflowResult) error {
	request := entry.Scenario.Request
	payload := request.Payload
	if options.Payload != nil {
		payload = options.Payload
	}
	if request.Delivery.Failure == "stale-timestamp" {
		logicalTime = logicalTime.Add(-24 * time.Hour)
	}
	generated, err := service.GenerateAt(app.PayloadOptions{
		EventType: request.Event.Type, EventVersion: request.Event.Version, Scenario: payload,
		Omit: request.Omit, Actors: entry.Scenario.Actors, ScenarioDefinitionID: entry.ID,
		ScenarioSourceVersion: entry.SourceVersion,
	}, first(options.SubscriptionID, request.Delivery.SubscriptionID), logicalTime)
	if err != nil {
		return err
	}
	generated, err = service.ApplyDeliveryFailure(generated, request.Delivery.Failure)
	if err != nil {
		return err
	}
	destination := first(options.Destination, request.Delivery.Destination)
	deliveries := options.Deliveries
	if deliveries == 0 {
		deliveries = request.Delivery.Attempts
	}
	if deliveries == 0 {
		deliveries = 1
	}
	statuses := options.ExpectedStatuses
	if len(statuses) == 0 {
		statuses = request.Delivery.Expect.Statuses
	}
	for iteration := 1; iteration <= deliveries; iteration++ {
		current := generated
		if iteration > 1 && !(options.SameMessageID || request.Delivery.SameMessageID) {
			current, err = service.GenerateAt(app.PayloadOptions{EventType: request.Event.Type, EventVersion: request.Event.Version, Scenario: payload, Omit: request.Omit, Actors: entry.Scenario.Actors, ScenarioDefinitionID: entry.ID, ScenarioSourceVersion: entry.SourceVersion}, first(options.SubscriptionID, request.Delivery.SubscriptionID), logicalTime)
			if err != nil {
				return err
			}
			current, err = service.ApplyDeliveryFailure(current, request.Delivery.Failure)
			if err != nil {
				return err
			}
		}
		deliveryResult, deliveryErr := service.Deliver(ctx, current, destination, options.DestinationURL, statuses)
		report.Deliveries = append(report.Deliveries, Delivery{Step: 1, Iteration: iteration, Result: deliveryResult})
		if deliveryErr != nil {
			return deliveryErr
		}
	}
	return nil
}

func runTimeline(ctx context.Context, service *app.Service, entry scenario.Entry, options Options, logicalTime time.Time, report *WorkflowResult) error {
	for stepIndex, step := range entry.Scenario.Steps {
		if step.Wait != "" {
			duration, _ := time.ParseDuration(step.Wait)
			logicalTime = logicalTime.Add(duration)
			if !options.SkipWait {
				if err := wait(ctx, duration); err != nil {
					return err
				}
			}
			continue
		}
		version := step.Version
		if version == 0 {
			version = 1
		}
		repeat := step.Repeat
		if repeat == 0 {
			repeat = 1
		}
		interval, _ := time.ParseDuration(step.Interval)
		for iteration := 1; iteration <= repeat; iteration++ {
			if iteration > 1 {
				logicalTime = logicalTime.Add(interval)
				if !options.SkipWait {
					if err := wait(ctx, interval); err != nil {
						return err
					}
				}
			}
			generated, err := service.GenerateAt(app.PayloadOptions{
				EventType: step.Event, EventVersion: version, Scenario: step.Payload, Omit: step.Omit,
				Actors: entry.Scenario.StepActors(step), ScenarioDefinitionID: entry.ID,
				ScenarioSourceVersion: entry.SourceVersion,
			}, options.SubscriptionID, logicalTime)
			if err != nil {
				return fmt.Errorf("step %d iteration %d: %w", stepIndex+1, iteration, err)
			}
			generated, err = service.ApplyDeliveryFailure(generated, step.Delivery.Failure)
			if err != nil {
				return fmt.Errorf("step %d iteration %d: %w", stepIndex+1, iteration, err)
			}
			destination := first(options.Destination, step.Delivery.Destination, entry.Scenario.Defaults.Destination)
			statuses := step.Delivery.Expect.Statuses
			if len(statuses) == 0 {
				statuses = entry.Scenario.Defaults.Expect.Statuses
			}
			deliveryResult, err := service.Deliver(ctx, generated, destination, options.DestinationURL, statuses)
			report.Deliveries = append(report.Deliveries, Delivery{Step: stepIndex + 1, Iteration: iteration, Result: deliveryResult})
			if err != nil {
				return fmt.Errorf("step %d iteration %d: %w", stepIndex+1, iteration, err)
			}
		}
	}
	return nil
}

func wait(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.Join(errors.New("workflow canceled"), ctx.Err())
	case <-timer.C:
		return nil
	}
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
