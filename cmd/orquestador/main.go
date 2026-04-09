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
	hostSintactica := "192.168.1.178:50051"
	archivo := "D:/Archivosm/m3438540.095"

	fmt.Printf("Conectando a Sintáctica (%s)...\n", hostSintactica)

	resultadoSintactica, err := grpcclient.ProcessFile(hostSintactica, archivo)
	if err != nil {
		log.Fatalf("Fallo crítico en Sintáctica: %v\n", err)
	}

	dataNode, ok := resultadoSintactica["data"].(map[string]interface{})
	if !ok {
		dataNode = resultadoSintactica
	}

	resultadoArr, ok := dataNode["Resultado"].([]interface{})
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

	fmt.Println("[OK] Documento base extraído con éxito.\n")

	modulos := []ModuleConfig{
		{
			Name:       "Simplificados",
			Host:       "192.168.1.178:50052",
			MethodName: "/apigrpc.SIMPLIFICADOS/SIMPLIFICADOS",
		},
		{
			Name:       "Fracciones",
			Host:       "192.168.1.178:50053",
			MethodName: "/apigrpc.FraccionesService/Fracciones",
		},
		{
			Name:       "IVA",
			Host:       "192.168.1.178:50054",
			MethodName: "/apigrpc.IvaService/Iva",
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	resultadosFinales := make(map[string]interface{})
	resultadosFinales["Sintactica"] = resultadoSintactica

	fmt.Println("Iniciando procesamiento de módulos")

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
	fmt.Println("Procesamiento paralelo finalizado\n")

	jsonResFinal, err := json.MarshalIndent(resultadosFinales, "", "  ")
	if err != nil {
		log.Fatalf("Error al serializar el resultado final: %v\n", err)
	}

	fmt.Println(" RESPUESTA FINAL ")
	fmt.Println(string(jsonResFinal))
}
