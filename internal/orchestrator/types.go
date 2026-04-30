package orchestrator

import "context"

type EventType string

const (
	EventLog           EventType = "log"
	EventSintactica    EventType = "sintactica"
	EventSimplificados EventType = "simplificados"
	EventNorContrib    EventType = "norcontrib"
	EventDTA           EventType = "dta"
	EventPedimento     EventType = "pedimento"
)

type PanelEvent struct {
	Type      EventType   `json:"type"`
	Pedimento string      `json:"pedimento,omitempty"`
	Data      interface{} `json:"data"`
}

// PanelWriter recibe eventos estructurados. Debe ser goroutine-safe.
type PanelWriter interface {
	Write(event PanelEvent)
}

type OrchestratorFn func(ctx context.Context, archivo string, writer PanelWriter) error
