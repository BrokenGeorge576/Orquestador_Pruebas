package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"orquestador_p/internal/grpcclient"
	"orquestador_p/internal/orchestrator"
)

const (
	urlSintactico = "https://prevalidador.appsrvr.dev/validacion-sintactica"
	urlLegacy     = "https://prevalidador-api.appsrvr.dev/nor-contribucion"
)

type ModuleConfig struct {
	Name       string
	Host       string
	MethodName string
}

type PedimentoContext struct {
	NumeroPedStr string
	Documento    map[string]interface{}
	EstadoSint   interface{}
}

// Agregar módulos aquí genera automáticamente un panel nuevo en la interfaz web.
var modulos = []ModuleConfig{
	{
		Name:       "Simplificados",
		Host:       "localhost:50051",
		MethodName: "/apigrpc.SIMPLIFICADOS/SIMPLIFICADOS",
	},
}

func runOrchestrator(ctx context.Context, archivo string, writer orchestrator.PanelWriter) error {
	logf := func(msg string, args ...interface{}) {
		writer.Write(orchestrator.PanelEvent{
			Type: orchestrator.EventLog,
			Data: fmt.Sprintf(msg, args...),
		})
	}

	// Notificar a la UI qué módulos gRPC existen para que cree los paneles.
	moduleNames := make([]string, len(modulos))
	for i, m := range modulos {
		moduleNames[i] = m.Name
	}
	writer.Write(orchestrator.PanelEvent{
		Type: orchestrator.EventModules,
		Data: moduleNames,
	})

	logf("Conectando a Sintáctica (%s)...", urlSintactico)
	resultadoSintactica, err := grpcclient.ProcessRestSintactica(urlSintactico, archivo)
	if err != nil {
		logf("ERROR: Fallo en Sintáctica: %v", err)
		return fmt.Errorf("fallo en Sintáctica: %w", err)
	}

	dataNode, ok := resultadoSintactica["data"].(map[string]interface{})
	if !ok {
		dataNode = resultadoSintactica
	}

	resultadoArr, ok := dataNode["Resultado"].([]interface{})
	if !ok || len(resultadoArr) == 0 {
		logf("ERROR: No se encontró 'Resultado' o está vacío")
		return fmt.Errorf("respuesta de Sintáctica sin Resultado")
	}
	logf("[OK] Documento extraído (%d pedimento(s))", len(resultadoArr))

	logf("Consultando NorContribuciones (%s)...", urlLegacy)
	catalogoTCambios, err := grpcclient.ProcessLegacyTCambio(urlLegacy, archivo)
	if err != nil {
		logf("ERROR: Fallo en NorContribuciones: %v", err)
		return fmt.Errorf("fallo en NorContribuciones: %w", err)
	}

	writer.Write(orchestrator.PanelEvent{
		Type: orchestrator.EventNorContrib,
		Data: catalogoTCambios,
	})
	logf("[OK] Datos de NorContribuciones recibidos")

	var pedimentosAProcesar []PedimentoContext

	for _, resItem := range resultadoArr {
		resMap := resItem.(map[string]interface{})
		documento, ok := resMap["Documento"].(map[string]interface{})
		if !ok {
			continue
		}

		inicioPed := documento["InicioPed"].(map[string]interface{})
		numeroPedFloat := inicioPed["NumeroPed"].(float64)
		numeroPedStr := strconv.FormatFloat(numeroPedFloat, 'f', 0, 64)

		dtaData := map[string]interface{}{}
		if extras, existe := catalogoTCambios[numeroPedStr]; existe {
			if tcambio, ok := extras["TCambio"]; ok {
				documento["TCambio"] = tcambio
				dtaData["TCambio"] = tcambio
			}
			if dta, ok := extras["DtaPartidas"]; ok {
				documento["DtaPartidas"] = dta
				dtaData["DtaPartidas"] = dta
			}
		}

		writer.Write(orchestrator.PanelEvent{
			Type:      orchestrator.EventDTA,
			Pedimento: numeroPedStr,
			Data:      dtaData,
		})

		writer.Write(orchestrator.PanelEvent{
			Type:      orchestrator.EventPedimento,
			Pedimento: numeroPedStr,
			Data:      documento,
		})

		var estadoSint interface{}
		if estados, ok := resMap["Estado"].([]interface{}); ok && len(estados) > 0 {
			estadoSint = estados[0]
		}

		writer.Write(orchestrator.PanelEvent{
			Type:      orchestrator.EventSintactica,
			Pedimento: numeroPedStr,
			Data:      estadoSint,
		})

		pedimentosAProcesar = append(pedimentosAProcesar, PedimentoContext{
			NumeroPedStr: numeroPedStr,
			Documento:    documento,
			EstadoSint:   estadoSint,
		})
	}

	logf("Procesando %d pedimento(s) con %d módulo(s) gRPC...", len(pedimentosAProcesar), len(modulos))

	for i, ped := range pedimentosAProcesar {
		select {
		case <-ctx.Done():
			logf("Procesamiento cancelado")
			return ctx.Err()
		default:
		}

		logf("Pedimento %d/%d (%s)...", i+1, len(pedimentosAProcesar), ped.NumeroPedStr)

		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, mod := range modulos {
			wg.Add(1)
			go func(m ModuleConfig, doc map[string]interface{}, numPed string) {
				defer wg.Done()
				res, err := grpcclient.ProcessDynamicModule(m.Host, m.MethodName, doc)

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					writer.Write(orchestrator.PanelEvent{
						Type:      orchestrator.EventLog,
						Data:      fmt.Sprintf("ADVERTENCIA: módulo '%s' falló (%s): %v", m.Name, m.Host, err),
					})
					writer.Write(orchestrator.PanelEvent{
						Type:      orchestrator.EventModule,
						Module:    m.Name,
						Pedimento: numPed,
						Data:      map[string]string{"status": "error", "message": err.Error()},
					})
				} else {
					writer.Write(orchestrator.PanelEvent{
						Type:      orchestrator.EventLog,
						Data:      fmt.Sprintf("[OK] Módulo '%s' — resultado recibido (ped. %s)", m.Name, numPed),
					})
					writer.Write(orchestrator.PanelEvent{
						Type:      orchestrator.EventModule,
						Module:    m.Name,
						Pedimento: numPed,
						Data:      res,
					})
				}
			}(mod, ped.Documento, ped.NumeroPedStr)
		}

		wg.Wait()
		logf("[OK] Pedimento %s completado", ped.NumeroPedStr)
	}

	logf("Procesamiento completo")
	return nil
}

// ── Batch Mode ─────────────────────────────────────────────────────────────

type batchCollector struct {
	mu      sync.Mutex
	results map[string][]orchestrator.BatchModuleResult
}

func newBatchCollector() *batchCollector {
	return &batchCollector{results: make(map[string][]orchestrator.BatchModuleResult)}
}

func (bc *batchCollector) Write(event orchestrator.PanelEvent) {
	if event.Type != orchestrator.EventModule {
		return
	}
	status := "ok"
	var data interface{}
	data = event.Data
	switch d := event.Data.(type) {
	case map[string]string:
		if d["status"] == "error" {
			status = "error"
		}
	case map[string]interface{}:
		topStatus, _ := d["status"].(string)
		if strings.EqualFold(topStatus, "error") || hasModuleError(d) {
			status = "error"
		}
	}
	bc.mu.Lock()
	bc.results[event.Module] = append(bc.results[event.Module], orchestrator.BatchModuleResult{
		Pedimento: event.Pedimento,
		Status:    status,
		Data:      data,
	})
	bc.mu.Unlock()
}

// hasModuleError inspecciona Resultado[*].Estado[*].Status para detectar errores
// de negocio que el módulo devuelve con Status:"Error" dentro de la respuesta gRPC.
func hasModuleError(data map[string]interface{}) bool {
	resultadoRaw, ok := data["Resultado"]
	if !ok {
		return false
	}
	resultado, ok := resultadoRaw.([]interface{})
	if !ok {
		return false
	}
	for _, itemRaw := range resultado {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		estados, ok := item["Estado"].([]interface{})
		if !ok {
			continue
		}
		for _, eRaw := range estados {
			e, ok := eRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if s, _ := e["Status"].(string); strings.EqualFold(s, "error") {
				return true
			}
		}
	}
	return false
}

func (bc *batchCollector) summary(fileName string) orchestrator.BatchFileSummary {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	var mods []orchestrator.BatchModuleSummary
	for _, mod := range modulos {
		results := bc.results[mod.Name]
		overall := "ok"
		for _, r := range results {
			if r.Status == "error" {
				overall = "error"
				break
			}
		}
		mods = append(mods, orchestrator.BatchModuleSummary{
			Module:        mod.Name,
			Results:       results,
			OverallStatus: overall,
		})
	}
	return orchestrator.BatchFileSummary{FileName: fileName, Modules: mods}
}

func runBatchOrchestrator(ctx context.Context, filePaths []string, writer orchestrator.PanelWriter) error {
	total := len(filePaths)
	for i, path := range filePaths {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		fileName := filepath.Base(path)
		writer.Write(orchestrator.PanelEvent{
			Type: orchestrator.EventBatchFileStart,
			Data: map[string]interface{}{"file_name": fileName, "index": i + 1, "total": total},
		})
		collector := newBatchCollector()
		runErr := runOrchestrator(ctx, path, collector)
		errMsg := ""
		if runErr != nil {
			errMsg = runErr.Error()
		}
		writer.Write(orchestrator.PanelEvent{
			Type: orchestrator.EventBatchFileDone,
			Data: map[string]interface{}{"summary": collector.summary(fileName), "error": errMsg},
		})
	}
	writer.Write(orchestrator.PanelEvent{
		Type: orchestrator.EventBatchDone,
		Data: map[string]interface{}{"total": total},
	})
	return nil
}
