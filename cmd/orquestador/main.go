package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"orquestador_p/internal/grpcclient"
)

type ModuleConfig struct {
	Name       string
	Host       string
	MethodName string
}

func main() {
	urlSintactico := "https://prevalidador.appsrvr.dev/validacion-sintactica"
	archivo := "/mnt/c/Users/DesarrolloGUAS/Documents/ArchivosM/m1698175.267"
	fmt.Printf("Conectando a Sintáctica (%s)...\n", urlSintactico)
	resultadoSintactica, err := grpcclient.ProcessRestSintactica(urlSintactico, archivo)
	if err != nil {
		log.Fatalf("Fallo crítico en Sintáctica: %v\n", err)
	}

	resultadoArr, ok := resultadoSintactica["Resultado"].([]interface{})
	if !ok || len(resultadoArr) == 0 {
		log.Fatalf("Error: No se encontró 'Resultado' o está vacío en la respuesta de Sintáctica.\n")
	}

	primerResultado, ok := resultadoArr[0].(map[string]interface{})
	if !ok {
		log.Fatalf("Error: El primer elemento de 'Resultado' no tiene un formato válido.\n")
	}

	documento, ok := primerResultado["Documento"].(map[string]interface{})
	if !ok {
		log.Fatalf("Error: No se encontró 'Documento' o no es un objeto válido.\n")
	}

	fmt.Println("[OK] Documento base extraído con éxito vía REST.\n")
	modulos := []ModuleConfig{
		{
			Name:       "Fracciones",
			Host:       "localhost:50053",
			MethodName: "/apigrpc.FraccionesService/Fracciones",
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	resultadosFinales := make(map[string]interface{})
	// resultadosFinales["Sintactica"] = resultadoSintactica

	fmt.Println("Iniciando procesamiento de módulos gRPC")

	for _, mod := range modulos {
		wg.Add(1)
		go func(m ModuleConfig) {
			defer wg.Done()
			res, err := grpcclient.ProcessDynamicModule(m.Host, m.MethodName, documento)

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
		}(mod)
	}

	wg.Wait()
	fmt.Println("Procesamiento \n")

	jsonResFinal, err := json.MarshalIndent(resultadosFinales, "", "  ")
	if err != nil {
		log.Fatalf("Error al serializar el resultado final: %v\n", err)
	}

	fmt.Println("=== RESPUESTA FINAL DEL ORQUESTADOR HÍBRIDO ===")
	fmt.Println(string(jsonResFinal))
}
