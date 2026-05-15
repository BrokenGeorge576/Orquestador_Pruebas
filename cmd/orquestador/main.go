package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"

	"orquestador_p/internal/grpcclient"
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

func main() {
	urlSintactico := "https://prevalidador.appsrvr.dev/validacion-sintactica"
	archivo := "m1609906.265"

	fmt.Printf("Conectando a Sintáctica (%s)...\n", urlSintactico)
	resultadoSintactica, err := grpcclient.ProcessRestSintactica(urlSintactico, archivo)
	if err != nil {
		log.Fatalf("Fallo en la ejecución de Sintáctica: %v\n", err)
	}

	dataNode, ok := resultadoSintactica["data"].(map[string]interface{})
	if !ok {
		dataNode = resultadoSintactica
	}

	resultadoArr, ok := dataNode["Resultado"].([]interface{})
	if !ok || len(resultadoArr) == 0 {
		log.Fatalf("Error: No se encontró 'Resultado' o está vacío.\n")
	}

	fmt.Println("[OK] Documento extraido.\n")

	urlLegacy := "https://prevalidador-api.appsrvr.dev/nor-contribucion"
	fmt.Printf("Consultando para datos extra (%s)...\n", urlLegacy)
	catalogoTCambios, err := grpcclient.ProcessLegacyTCambio(urlLegacy, archivo)
	if err != nil {
		log.Fatalf("Fallo al procesar Simplificados: %v\n", err)
	}
	fmt.Println("[OK] Datos extras anexados exitosamente.\n")

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

		if extras, existe := catalogoTCambios[numeroPedStr]; existe {
			if tcambio, ok := extras["TCambio"]; ok {
				documento["TCambio"] = tcambio
			}
			if dta, ok := extras["DtaPartidas"]; ok {
				documento["DtaPartidas"] = dta
			}
		}

		var estadoSint interface{}
		if estados, ok := resMap["Estado"].([]interface{}); ok && len(estados) > 0 {
			estadoSint = estados[0]
		}

		pedimentosAProcesar = append(pedimentosAProcesar, PedimentoContext{
			NumeroPedStr: numeroPedStr,
			Documento:    documento,
			EstadoSint:   estadoSint,
		})
	}

	modulos := []ModuleConfig{
		{
			Name:       "Fracciones",
			Host:       "localhost:50053",
			MethodName: "/apigrpc.FraccionesService/Fracciones",
		},
	}

	fmt.Println("Iniciando procesamiento de módulo")

	for i, ped := range pedimentosAProcesar {
		fmt.Printf("\n>>> Procesando Pedimento %d de %d (Pedimento: %s) <<<\n", i+1, len(pedimentosAProcesar), ped.NumeroPedStr)

		// Debug del pedimento
		docJSON, err := json.MarshalIndent(ped.Documento, "", "  ")
		if err != nil {
			fmt.Printf("[ADVERTENCIA] No se pudo serializar el pedimento %s para debug: %v\n", ped.NumeroPedStr, err)
		} else {
			fmt.Printf("=== PEDIMENTO %s ===\n", ped.NumeroPedStr)
			fmt.Println(string(docJSON))
			fmt.Println("============================================")
		}
		// Fin de debug
		var wg sync.WaitGroup
		var mu sync.Mutex
		resultadosFinales := make(map[string]interface{})

		if ped.EstadoSint != nil {
			resultadosFinales["Sintactica"] = ped.EstadoSint
		}

		for _, mod := range modulos {
			wg.Add(1)
			go func(m ModuleConfig, doc map[string]interface{}) {
				defer wg.Done()
				res, err := grpcclient.ProcessDynamicModule(m.Host, m.MethodName, doc)

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					fmt.Printf("[ADVERTENCIA] Módulo '%s' falló o no está disponible (%s): %v\n", m.Name, m.Host, err)
					resultadosFinales[m.Name] = map[string]string{
						"status":  "error",
						"message": err.Error(),
					}
				} else {
					fmt.Printf("[OK] Respuesta exitosa recibida de '%s'\n", m.Name)
					resultadosFinales[m.Name] = res
				}
			}(mod, ped.Documento)
		}

		wg.Wait()

		jsonResFinal, err := json.MarshalIndent(resultadosFinales, "", "  ")
		if err != nil {
			log.Fatalf("Error al serializar el resultado final: %v\n", err)
		}

		fmt.Printf("=== RESPUESTA PARA EL PEDIMENTO %s ===\n", ped.NumeroPedStr)
		fmt.Println(string(jsonResFinal))
	}
}
