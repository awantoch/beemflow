package model

import (
	"time"

	"github.com/google/uuid"
)

type Flow struct {
	Name    string                 `yaml:"name" json:"name"`
	Version string                 `yaml:"version,omitempty" json:"version,omitempty"`
	On      interface{}            `yaml:"on" json:"on,omitempty"`
	Vars    map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
	Steps   []Step                 `yaml:"steps" json:"steps"`
	Catch   []Step                 `yaml:"catch,omitempty" json:"catch,omitempty"`
}

type Step struct {
	ID         string                 `yaml:"id" json:"id"`
	Use        string                 `yaml:"use,omitempty" json:"use,omitempty"`
	With       map[string]interface{} `yaml:"with,omitempty" json:"with,omitempty"`
	DependsOn  []string               `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Parallel   bool                   `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	If         string                 `yaml:"if,omitempty" json:"if,omitempty"`
	Foreach    string                 `yaml:"foreach,omitempty" json:"foreach,omitempty"`
	As         string                 `yaml:"as,omitempty" json:"as,omitempty"`
	Do         []Step                 `yaml:"do,omitempty" json:"do,omitempty"`
	Steps      []Step                 `yaml:"steps,omitempty" json:"steps,omitempty"`
	Retry      *RetrySpec             `yaml:"retry,omitempty" json:"retry,omitempty"`
	AwaitEvent *AwaitEventSpec        `yaml:"await_event,omitempty" json:"await_event,omitempty"`
	Wait       *WaitSpec              `yaml:"wait,omitempty" json:"wait,omitempty"`
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
	ID        uuid.UUID              `json:"id"`
	FlowName  string                 `json:"flow_name"`
	Event     map[string]interface{} `json:"event"`
	Vars      map[string]interface{} `json:"vars"`
	Status    RunStatus              `json:"status"`
	StartedAt time.Time              `json:"started_at"`
	EndedAt   *time.Time             `json:"ended_at,omitempty"`
	Steps     []StepRun              `json:"steps"`
}

type StepRun struct {
	ID        uuid.UUID              `json:"id"`
	RunID     uuid.UUID              `json:"run_id"`
	StepName  string                 `json:"step_name"`
	Status    StepStatus             `json:"status"`
	StartedAt time.Time              `json:"started_at"`
	EndedAt   *time.Time             `json:"ended_at,omitempty"`
	Outputs   map[string]interface{} `json:"outputs,omitempty"`
	Error     string                 `json:"error,omitempty"`
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
