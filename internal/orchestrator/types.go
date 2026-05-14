package orchestrator

import "context"

type EventType string

const (
	EventLog       EventType = "log"
	EventSintactica EventType = "sintactica"
	EventNorContrib EventType = "norcontrib"
	EventDTA       EventType = "dta"
	EventPedimento EventType = "pedimento"
	EventModules   EventType = "modules" // lista de módulos al inicio del stream
	EventModule    EventType = "module"  // resultado de un módulo gRPC (campo Module indica cuál)
)

type PanelEvent struct {
	Type      EventType   `json:"type"`
	Pedimento string      `json:"pedimento,omitempty"`
	Module    string      `json:"module,omitempty"`
	Data      interface{} `json:"data"`
}

// PanelWriter recibe eventos estructurados. Debe ser goroutine-safe.
type PanelWriter interface {
	Write(event PanelEvent)
}

type OrchestratorFn func(ctx context.Context, archivo string, writer PanelWriter) error

const (
	EventBatchFileStart EventType = "batch_file_start"
	EventBatchFileDone  EventType = "batch_file_done"
	EventBatchDone      EventType = "batch_done"
)

type BatchOrchestratorFn func(ctx context.Context, filePaths []string, writer PanelWriter) error

type BatchModuleResult struct {
	Pedimento string      `json:"pedimento"`
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
}

type BatchModuleSummary struct {
	Module        string              `json:"module"`
	Results       []BatchModuleResult `json:"results"`
	OverallStatus string              `json:"overall_status"`
}

type BatchFileSummary struct {
	FileName string               `json:"file_name"`
	Modules  []BatchModuleSummary `json:"modules"`
}
