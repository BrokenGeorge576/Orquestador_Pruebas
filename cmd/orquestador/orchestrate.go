package main

import (
	"context"
	"fmt"
	"strconv"
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

	logf("Procesando %d pedimento(s) con módulos gRPC...", len(pedimentosAProcesar))

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
						Type:      orchestrator.EventSimplificados,
						Pedimento: numPed,
						Data: map[string]string{
							"status":  "error",
							"message": err.Error(),
						},
					})
				} else {
					writer.Write(orchestrator.PanelEvent{
						Type:      orchestrator.EventSimplificados,
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
