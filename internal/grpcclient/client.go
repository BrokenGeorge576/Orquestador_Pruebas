package grpcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

func ProcessRestSintactica(targetUrl string, filePath string) (map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo local: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("error al crear el form-data: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("error copiando el archivo al buffer: %v", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", targetUrl, body)
	if err != nil {
		return nil, fmt.Errorf("error creando request HTTP: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 360 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error ejecutando petición REST a Sintáctica: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("el servidor respondió con status no exitoso: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decodificando el JSON de Sintáctica: %v", err)
	}

	return result, nil
}

func ProcessDynamicModule(host string, fullMethodName string, reqData map[string]interface{}) (map[string]interface{}, error) {
	reqStruct, err := structpb.NewStruct(reqData)
	if err != nil {
		return nil, fmt.Errorf("error serialización: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	outStruct := new(structpb.Struct)

	err = conn.Invoke(ctx, fullMethodName, reqStruct, outStruct)
	if err != nil {
		return nil, err
	}

	return outStruct.AsMap(), nil
}

func ProcessLegacyTCambio(targetUrl string, filePath string) (map[string]map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("error al crear form-data: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("error copiando archivo al buffer: %v", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", targetUrl, body)
	if err != nil {
		return nil, fmt.Errorf("error creando request a api: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error en petición a api: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api respondió con status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decodificando json: %v", err)
	}

	resultados, ok := result["resultado"].([]interface{})
	if !ok || len(resultados) == 0 {
		return nil, fmt.Errorf("estructura 'resultado' no encontrada o vacía")
	}

	catalogoExtras := make(map[string]map[string]interface{})

	for _, resItem := range resultados {
		resMap, ok := resItem.(map[string]interface{})
		if !ok {
			continue
		}

		numPedimento, ok := resMap["pedimento"].(string)
		if !ok {
			continue
		}

		estados, ok := resMap["estado"].([]interface{})
		if !ok {
			continue
		}

		pedimentoExtras := make(map[string]interface{})

		for _, e := range estados {
			estadoMap, ok := e.(map[string]interface{})
			if !ok {
				continue
			}

			// Lógica TCambio
			if estadoMap["id"] == "tcambio" {
				tcambioStr, _ := estadoMap["tcambio"].(string)
				fAplicaStr, _ := estadoMap["f_aplica"].(string)
				factorStr, _ := estadoMap["factor"].(string)

				tcambioFloat, _ := strconv.ParseFloat(tcambioStr, 64)
				factorInt, _ := strconv.Atoi(factorStr)

				pedimentoExtras["TCambio"] = map[string]interface{}{
					"TipoCambio": tcambioFloat,
					"FAplica":    fAplicaStr,
					"Factor":     factorInt,
				}
			}

			if estadoMap["id"] == "dta" {
				if dtaJson, ok := estadoMap["json"].(map[string]interface{}); ok {
					dtaMapeado := make(map[string]interface{})

					dtaMapeado["TipoDta"] = dtaJson["tipo_dta"]
					dtaMapeado["TodasExcentas"] = dtaJson["todas_excentas"]
					dtaMapeado["FactorActualizacion"] = dtaJson["factor_actualizacion"]
					dtaMapeado["DtaActualiza"] = dtaJson["dta_actualiza"]
					dtaMapeado["FechaAplica"] = dtaJson["fecha_aplica"]
					dtaMapeado["Remesas"] = dtaJson["remesas"]

					var partidasMapeadas []interface{}
					if partidasRaw, ok := dtaJson["partidas"].([]interface{}); ok {
						for _, pRaw := range partidasRaw {
							if pMap, ok := pRaw.(map[string]interface{}); ok {
								pMapeada := make(map[string]interface{})
								pMapeada["FraccionArancelaria"] = pMap["fraccion_arancelaria"]
								pMapeada["ValorAduana"] = pMap["valor_aduana"]
								pMapeada["NumPartida"] = pMap["num_partida"]
								pMapeada["IncluyeCertificadoElegibilidad"] = pMap["incluye_certificado_elegibilidad"]
								pMapeada["PaisOrigenDestinoMex"] = pMap["pais_origen_destino_mex"]
								pMapeada["IncluyeCodigoGenerico"] = pMap["incluye_codigo_generico"]
								pMapeada["IncluyeTratadosComerciales"] = pMap["incluye_tratados_comerciales"]
								pMapeada["IncluyeTratadosCuotaFija"] = pMap["incluye_tratados_cuota_fija"]
								pMapeada["IncluyeAlMillar"] = pMap["incluye_al_millar"]

								partidasMapeadas = append(partidasMapeadas, pMapeada)
							}
						}
					}
					dtaMapeado["Partidas"] = partidasMapeadas
					pedimentoExtras["DtaPartidas"] = dtaMapeado
				}
			}
		}

		catalogoExtras[numPedimento] = pedimentoExtras
	}

	return catalogoExtras, nil
}
