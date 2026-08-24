package suite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/workflow"
)

const FormatVersion = 1

type Suite struct {
	Version     int        `yaml:"version" json:"version"`
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Thresholds  Thresholds `yaml:"thresholds,omitempty" json:"thresholds"`
	Cases       []Case     `yaml:"cases" json:"cases"`
}

type Thresholds struct {
	MaxFailures      int     `yaml:"maxFailures,omitempty" json:"maxFailures"`
	MinPassedPercent float64 `yaml:"minPassedPercent,omitempty" json:"minPassedPercent,omitempty"`
}

type Case struct {
	Name     string      `yaml:"name,omitempty" json:"name,omitempty"`
	Scenario string      `yaml:"scenario" json:"scenario"`
	Expect   Expectation `yaml:"expect" json:"expect"`
}

type Expectation struct {
	Deliveries    int   `yaml:"deliveries,omitempty" json:"deliveries,omitempty"`
	SameMessageID bool  `yaml:"sameMessageId,omitempty" json:"sameMessageId,omitempty"`
	Statuses      []int `yaml:"statuses" json:"statuses"`
}

type RunOptions struct {
	Destination    string
	DestinationURL string
	Start          time.Time
	SkipWait       bool
}

type SuiteResult struct {
	SuiteID       string       `json:"suiteId"`
	Name          string       `json:"name"`
	Passed        bool         `json:"passed"`
	PassedCases   int          `json:"passedCases"`
	FailedCases   int          `json:"failedCases"`
	PassedPercent float64      `json:"passedPercent"`
	StartedAt     time.Time    `json:"startedAt"`
	EndedAt       time.Time    `json:"endedAt"`
	Cases         []CaseResult `json:"cases"`
	Thresholds    Thresholds   `json:"thresholds"`
}

type CaseResult struct {
	Index      int                     `json:"index"`
	Name       string                  `json:"name"`
	ScenarioID string                  `json:"scenarioId"`
	Passed     bool                    `json:"passed"`
	Workflow   workflow.WorkflowResult `json:"workflow"`
	Error      string                  `json:"error,omitempty"`
}

func (value Suite) Validate(scenarios *scenario.Store) error {
	var problems []error
	if value.Version != FormatVersion {
		problems = append(problems, fmt.Errorf("unsupported suite version %d", value.Version))
	}
	if value.Name == "" {
		problems = append(problems, errors.New("suite name is required"))
	}
	if len(value.Cases) == 0 {
		problems = append(problems, errors.New("suite requires at least one case"))
	}
	if value.Thresholds.MaxFailures < 0 {
		problems = append(problems, errors.New("suite maxFailures cannot be negative"))
	}
	if value.Thresholds.MinPassedPercent < 0 || value.Thresholds.MinPassedPercent > 100 {
		problems = append(problems, errors.New("suite minPassedPercent must be between 0 and 100"))
	}
	for index, testCase := range value.Cases {
		entry, err := scenarios.Get(testCase.Scenario)
		if err != nil {
			problems = append(problems, fmt.Errorf("case %d: %w", index+1, err))
			continue
		}
		if err := scenarios.Validate(entry); err != nil {
			problems = append(problems, fmt.Errorf("case %d: %w", index+1, err))
		}
		if testCase.Expect.Deliveries < 0 {
			problems = append(problems, fmt.Errorf("case %d deliveries must be positive", index+1))
		}
		for _, status := range testCase.Expect.Statuses {
			if status < 100 || status > 599 {
				problems = append(problems, fmt.Errorf("case %d has invalid HTTP status %d", index+1, status))
			}
		}
	}
	return errors.Join(problems...)
}

func Run(ctx context.Context, service *app.Service, scenarios *scenario.Store, entry Entry, options RunOptions) (SuiteResult, error) {
	startedAt := options.Start.UTC()
	if startedAt.IsZero() {
		startedAt = service.Now().UTC()
	}
	report := SuiteResult{SuiteID: entry.ID, Name: entry.Suite.Name, Passed: true, StartedAt: startedAt, Thresholds: entry.Suite.Thresholds}
	for index, testCase := range entry.Suite.Cases {
		scenarioEntry, err := scenarios.Get(testCase.Scenario)
		caseName := testCase.Name
		if caseName == "" {
			caseName = testCase.Scenario
		}
		caseResult := CaseResult{Index: index + 1, Name: caseName, ScenarioID: testCase.Scenario}
		if err == nil {
			caseResult.Workflow, err = workflow.Run(ctx, service, scenarioEntry, workflow.Options{
				Destination: options.Destination, DestinationURL: options.DestinationURL,
				Start: startedAt, SkipWait: options.SkipWait, Deliveries: testCase.Expect.Deliveries,
				SameMessageID: testCase.Expect.SameMessageID, ExpectedStatuses: testCase.Expect.Statuses,
			})
		}
		caseResult.Passed = err == nil
		if err == nil && testCase.Expect.SameMessageID && !sameMessageID(caseResult.Workflow) {
			err = errors.New("deliveries did not reuse the same message ID")
			caseResult.Passed = false
		}
		if err != nil {
			caseResult.Error = err.Error()
			report.FailedCases++
		} else {
			report.PassedCases++
		}
		report.Cases = append(report.Cases, caseResult)
	}
	total := report.PassedCases + report.FailedCases
	if total > 0 {
		report.PassedPercent = float64(report.PassedCases) * 100 / float64(total)
	}
	report.Passed = report.FailedCases <= entry.Suite.Thresholds.MaxFailures
	if entry.Suite.Thresholds.MinPassedPercent > 0 && report.PassedPercent < entry.Suite.Thresholds.MinPassedPercent {
		report.Passed = false
	}
	report.EndedAt = service.Now().UTC()
	if !report.Passed {
		return report, fmt.Errorf("suite thresholds failed: %d failed case(s), %.1f%% passed", report.FailedCases, report.PassedPercent)
	}
	return report, nil
}

func sameMessageID(result workflow.WorkflowResult) bool {
	if len(result.Deliveries) < 2 {
		return false
	}
	id := result.Deliveries[0].Result.MessageID
	for _, delivery := range result.Deliveries[1:] {
		if delivery.Result.MessageID != id {
			return false
		}
	}
	return true
}
