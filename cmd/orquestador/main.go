package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"orquestador_p/internal/orchestrator"
	"orquestador_p/internal/webserver"
)

// stdoutWriter imprime los eventos en terminal, igual que el main() original.
type stdoutWriter struct{}

func (stdoutWriter) Write(ev orchestrator.PanelEvent) {
	switch ev.Type {
	case orchestrator.EventLog:
		fmt.Println(ev.Data)
	case orchestrator.EventSintactica:
		fmt.Printf("\n=== SINTÁCTICO — Pedimento %s ===\n", ev.Pedimento)
		printJSON(ev.Data)
	case orchestrator.EventPedimento:
		fmt.Printf("\n=== PEDIMENTO %s ===\n", ev.Pedimento)
		printJSON(ev.Data)
		fmt.Println("============================================")
	case orchestrator.EventNorContrib:
		fmt.Println("\n=== NOR CONTRIBUCIONES ===")
		printJSON(ev.Data)
	case orchestrator.EventDTA:
		fmt.Printf("\n=== DTA/TCAMBIO — Pedimento %s ===\n", ev.Pedimento)
		printJSON(ev.Data)
	case orchestrator.EventModules:
		// solo informativo en terminal
	case orchestrator.EventModule:
		fmt.Printf("\n=== %s — Pedimento %s ===\n", ev.Module, ev.Pedimento)
		printJSON(ev.Data)
	}
}

func printJSON(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%v\n", v)
		return
	}
	fmt.Println(string(b))
}

func main() {
	file := flag.String("file", "", "Archivo m a procesar (modo terminal)")
	port := flag.String("port", "8080", "Puerto del servidor web")
	flag.Parse()

	if *file != "" {
		if err := runOrchestrator(context.Background(), *file, stdoutWriter{}); err != nil {
			log.Fatalf("Error: %v\n", err)
		}
		return
	}

	srv := webserver.New(runOrchestrator, runBatchOrchestrator, staticFiles)
	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Orquestador iniciado → http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, srv))
}
