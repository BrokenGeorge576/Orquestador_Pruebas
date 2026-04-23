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

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("error cerrando el writer multipart: %v", err)
	}

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
