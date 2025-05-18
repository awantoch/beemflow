package model

import (
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Flow struct {
	Name    string                 `yaml:"name" json:"name"`
	Version string                 `yaml:"version,omitempty" json:"version,omitempty"`
	On      interface{}            `yaml:"on" json:"on"`
	Vars    map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
	Steps   []Step                 `yaml:"steps" json:"steps"`
	Catch   map[string]Step        `yaml:"catch,omitempty" json:"catch,omitempty"`
}

type Step struct {
	ID        string                 `yaml:"id" json:"id"`
	Use       string                 `yaml:"use,omitempty" json:"use,omitempty"`
	With      map[string]interface{} `yaml:"with,omitempty" json:"with,omitempty"`
	DependsOn []string               `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	// Deprecated: use ParallelBool and ParallelSteps
	Parallel      bool            `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	ParallelBool  bool            `yaml:"-" json:"-"` // true if parallel: true
	ParallelSteps []string        `yaml:"-" json:"-"` // set if parallel: [id,...]
	If            string          `yaml:"if,omitempty" json:"if,omitempty"`
	Foreach       string          `yaml:"foreach,omitempty" json:"foreach,omitempty"`
	As            string          `yaml:"as,omitempty" json:"as,omitempty"`
	Do            []Step          `yaml:"do,omitempty" json:"do,omitempty"`
	Steps         []Step          `yaml:"steps,omitempty" json:"steps,omitempty"`
	Retry         *RetrySpec      `yaml:"retry,omitempty" json:"retry,omitempty"`
	AwaitEvent    *AwaitEventSpec `yaml:"await_event,omitempty" json:"await_event,omitempty"`
	Wait          *WaitSpec       `yaml:"wait,omitempty" json:"wait,omitempty"`
}

func (s *Step) UnmarshalYAML(value *yaml.Node) error {
	type stepAlias Step // prevent recursion
	var raw stepAlias
	if err := value.Decode(&raw); err != nil {
		return err
	}
	// Now handle 'parallel' manually
	for i := 0; i < len(value.Content); i += 2 {
		k := value.Content[i]
		if k.Value == "parallel" {
			v := value.Content[i+1]
			switch v.Kind {
			case yaml.ScalarNode:
				var b bool
				if err := v.Decode(&b); err == nil {
					raw.ParallelBool = b
				}
			case yaml.SequenceNode:
				var arr []string
				if err := v.Decode(&arr); err == nil {
					raw.ParallelSteps = arr
				}
			}
		}
	}
	*(*Step)(s) = Step(raw)
	return nil
}

type RetrySpec struct {
	Attempts int `yaml:"attempts" json:"attempts"`
	DelaySec int `yaml:"delay_sec" json:"delay_sec"`
}

type AwaitEventSpec struct {
	Source  string                 `yaml:"source" json:"source"`
	Match   map[string]interface{} `yaml:"match" json:"match"`
	Timeout string                 `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

type WaitSpec struct {
	Seconds int    `yaml:"seconds,omitempty" json:"seconds,omitempty"`
	Until   string `yaml:"until,omitempty" json:"until,omitempty"`
}

type Run struct {
	ID        uuid.UUID      `json:"id"`
	FlowName  string         `json:"flow_name"`
	Event     map[string]any `json:"event"`
	Vars      map[string]any `json:"vars"`
	Status    RunStatus      `json:"status"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   *time.Time     `json:"ended_at,omitempty"`
	Steps     []StepRun      `json:"steps"`
}

type StepRun struct {
	ID        uuid.UUID      `json:"id"`
	RunID     uuid.UUID      `json:"run_id"`
	StepName  string         `json:"step_name"`
	Status    StepStatus     `json:"status"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   *time.Time     `json:"ended_at,omitempty"`
	Outputs   map[string]any `json:"outputs,omitempty"`
	Error     string         `json:"error,omitempty"`
}

type RunStatus string

type StepStatus string

const (
	RunPending   RunStatus = "PENDING"
	RunRunning   RunStatus = "RUNNING"
	RunSucceeded RunStatus = "SUCCEEDED"
	RunFailed    RunStatus = "FAILED"
	RunWaiting   RunStatus = "WAITING"
	RunSkipped   RunStatus = "SKIPPED"

	StepPending   StepStatus = "PENDING"
	StepRunning   StepStatus = "RUNNING"
	StepSucceeded StepStatus = "SUCCEEDED"
	StepFailed    StepStatus = "FAILED"
	StepWaiting   StepStatus = "WAITING"
)
