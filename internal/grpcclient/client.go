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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

type DtaPartidaItem struct {
	FraccionArancelaria            string `json:"FraccionArancelaria"`
	NumPartida                     string `json:"NumPartida"`
	IncluyeAlMillar                bool   `json:"IncluyeAlMillar"`
	IncluyeCertificadoElegibilidad bool   `json:"IncluyeCertificadoElegibilidad"`
	IncluyeCodigoGenerico          bool   `json:"IncluyeCodigoGenerico"`
	IncluyeTratadosComerciales     bool   `json:"IncluyeTratadosComerciales"`
	IncluyeTratadosCuotaFija       bool   `json:"IncluyeTratadosCuotaFija"`
	PaisOrigenDestinoMex           bool   `json:"PaisOrigenDestinoMex"`
}

type DtaPartidas struct {
	FechaAplica         string           `json:"FechaAplica"`
	DtaActualiza        bool             `json:"DtaActualiza"`
	FactorActualizacion string           `json:"FactorActualizacion"`
	TipoDta             []any            `json:"TipoDta"`
	Partidas            []DtaPartidaItem `json:"Partidas"`
}

type norContribucionRespuesta struct {
	Resultado []struct {
		Pedimento string `json:"pedimento"`
		Estado    []struct {
			ID     string          `json:"id"`
			Status string          `json:"status"`
			JSON   json.RawMessage `json:"json,omitempty"`
		} `json:"estado"`
	} `json:"resultado"`
}

type dtaJSON struct {
	TipoDta             []any        `json:"tipo_dta"`
	FactorActualizacion string       `json:"factor_actualizacion"`
	DtaActualiza        bool         `json:"dta_actualiza"`
	FechaAplica         string       `json:"fecha_aplica"`
	Partidas            []dtaPartida `json:"partidas"`
}

type dtaPartida struct {
	FraccionArancelaria            string `json:"fraccion_arancelaria"`
	NumPartida                     string `json:"num_partida"`
	IncluyeAlMillar                bool   `json:"incluye_al_millar"`
	IncluyeCertificadoElegibilidad bool   `json:"incluye_certificado_elegibilidad"`
	IncluyeCodigoGenerico          bool   `json:"incluye_codigo_generico"`
	IncluyeTratadosComerciales     bool   `json:"incluye_tratados_comerciales"`
	IncluyeTratadosCuotaFija       bool   `json:"incluye_tratados_cuota_fija"`
	PaisOrigenDestinoMex           bool   `json:"pais_origen_destino_mex"`
}

func ProcessRestNorContribucion(targetUrl string, filePath string) (map[string]DtaPartidas, error) {
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
	if _, err = io.Copy(part, file); err != nil {
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
		return nil, fmt.Errorf("error ejecutando petición REST a NorContribucion: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NorContribucion respondió con status no exitoso: %d", resp.StatusCode)
	}

	var raw norContribucionRespuesta
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("error decodificando JSON de NorContribucion: %v", err)
	}

	resultado := make(map[string]DtaPartidas)

	for _, item := range raw.Resultado {
		for _, estado := range item.Estado {
			if estado.ID != "dta" {
				continue
			}
			if estado.JSON == nil {
				break
			}

			var dta dtaJSON
			if err := json.Unmarshal(estado.JSON, &dta); err != nil {
				fmt.Printf("[ADVERTENCIA] No se pudo parsear nodo dta del pedimento %s: %v\n", item.Pedimento, err)
				break
			}

			partidas := make([]DtaPartidaItem, 0, len(dta.Partidas))
			for _, p := range dta.Partidas {
				partidas = append(partidas, DtaPartidaItem{
					FraccionArancelaria:            p.FraccionArancelaria,
					NumPartida:                     p.NumPartida,
					IncluyeAlMillar:                p.IncluyeAlMillar,
					IncluyeCertificadoElegibilidad: p.IncluyeCertificadoElegibilidad,
					IncluyeCodigoGenerico:          p.IncluyeCodigoGenerico,
					IncluyeTratadosComerciales:     p.IncluyeTratadosComerciales,
					IncluyeTratadosCuotaFija:       p.IncluyeTratadosCuotaFija,
					PaisOrigenDestinoMex:           p.PaisOrigenDestinoMex,
				})
			}

			resultado[item.Pedimento] = DtaPartidas{
				FechaAplica:         dta.FechaAplica,
				DtaActualiza:        dta.DtaActualiza,
				FactorActualizacion: dta.FactorActualizacion,
				TipoDta:             dta.TipoDta,
				Partidas:            partidas,
			}
			break
		}
	}

	return resultado, nil
}

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

func ProcessGRPCTCambio(host string, reqData map[string]interface{}) (map[string]interface{}, error) {
	reqStruct, err := structpb.NewStruct(reqData)
	if err != nil {
		return nil, fmt.Errorf("error serializando request TCambio: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("error conectando gRPC TCambio: %v", err)
	}
	defer conn.Close()

	outStruct := new(structpb.Struct)
	if err = conn.Invoke(ctx, "/apigrpc.TCambioService/TCambio", reqStruct, outStruct); err != nil {
		return nil, err
	}

	result := outStruct.AsMap()

	resultados, ok := result["Resultado"].([]interface{})
	if !ok || len(resultados) == 0 {
		return nil, fmt.Errorf("estructura 'Resultado' no encontrada o vacía")
	}

	primer, ok := resultados[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("primer elemento de 'Resultado' inválido")
	}

	datosExtra, ok := primer["DatosExtra"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'DatosExtra' no encontrado en la respuesta")
	}

	extras := make(map[string]interface{})
	if tcambio, ok := datosExtra["TCambio"]; ok {
		extras["TCambio"] = tcambio
	}
	if dta, ok := datosExtra["DtaPartidas"]; ok {
		extras["DtaPartidas"] = dta
	}

	return extras, nil
}
