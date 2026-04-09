package main

import (
	"encoding/json"
	"fmt"
	"log"

	"orquestador_p/internal/grpcclient"
)

func main() {
	hostSintactica := "192.168.1.178:50051"
	hostSimplificados := "192.168.1.178:50052"
	hostFracciones := "192.168.1.178:50053"

	archivo := "D:/Archivosm/m3438540.095"

	fmt.Printf("Conectando a Sintáctica (%s)\n", hostSintactica)
	resultadoSintactica, err := grpcclient.ProcessFile(hostSintactica, archivo)
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

	primerResultado, ok := resultadoArr[0].(map[string]interface{})
	if !ok {
		log.Fatalf("Error: El primer elemento de 'Resultado' no es válido.\n")
	}

	documento, ok := primerResultado["Documento"].(map[string]interface{})
	if !ok {
		log.Fatalf("Error: No se encontró 'Documento' o no tiene el formato de objeto.\n")
	}

	fmt.Println("Documento extraído con éxito.\n")

	fmt.Printf("Conectando a Simplificados (%s)\n", hostSimplificados)
	resultadoSimplificados, err := grpcclient.ProcessSimplificados(hostSimplificados, documento)
	if err != nil {
		log.Fatalf("Fallo al procesar Simplificados: %v\n", err)
	}

	jsonResSimplificados, _ := json.MarshalIndent(resultadoSimplificados, "", "  ")
	fmt.Println("Respuesta de Simplificados:")
	fmt.Println(string(jsonResSimplificados) + "\n")

	fmt.Printf("Conectando a Fracciones (%s)\n", hostFracciones)
	resultadoFracciones, err := grpcclient.ProcessFracciones(hostFracciones, documento)
	if err != nil {
		log.Fatalf("Fallo al procesar Fracciones: %v\n", err)
	}

	jsonResFracciones, _ := json.MarshalIndent(resultadoFracciones, "", "  ")
	fmt.Println("Respuesta de Fracciones:")
	fmt.Println(string(jsonResFracciones) + "\n")
}
