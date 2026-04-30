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
