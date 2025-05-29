package constants

// Event Bus Topics and Prefixes
const (
	// Event topic prefix for resume operations
	EventTopicResumePrefix = "resume:"
)

// Error Messages for Engine Operations
const (
	// Await event error messages
	ErrAwaitEventMissingToken  = "await_event step missing or invalid token in match"
	ErrAwaitEventPause         = "await_event pause"
	ErrStepWaitingForEvent     = "step %s is waiting for event (await_event pause)"
	ErrFailedToRenderToken     = "failed to render token template: %w"
	ErrFailedToDeletePausedRun = "Failed to delete paused run during resume: %v"
	ErrFailedToPersistStep     = "Failed to persist step result: %v"
	ErrSaveRunFailed           = "SaveRun failed: %v"
)

// Context Keys and Match Keys
const (
	// Match keys for await_event steps
	MatchKeyToken = "token"
)

// Template Keys and Field Names
const (
	// Standard template data keys
	TemplateKeyEvent   = "event"
	TemplateKeyVars    = "vars"
	TemplateKeyOutputs = "outputs"
	TemplateKeySecrets = "secrets"
)

// Step Types and Execution Modes
const (
	// Step execution types
	StepTypeAwaitEvent = "await_event"
	StepTypeParallel   = "parallel"
	StepTypeSequential = "sequential"
	StepTypeForeach    = "foreach"
)

// Registry and Storage Related
const (
	// Default registry configuration
	DefaultLocalRegistryFile = ".beemflow/registry.json"
)
