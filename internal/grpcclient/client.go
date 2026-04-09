package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	pb "orquestador_p/pb"
)

func ProcessFile(host string, filePath string) (map[string]interface{}, error) {
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("error conectando al host %s: %v", host, err)
	}
	defer conn.Close()

	client := pb.NewFileProcessorClient(conn)

	ctx := context.Background()

	req := &pb.FileRequest{FilePath: filePath}
	res, err := client.ProcessFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error en llamada gRPC a ProcessFile: %v", err)
	}

	if res.Data != nil {
		return res.Data.AsMap(), nil
	}

	return nil, nil
}

func ProcessSimplificados(host string, reqData map[string]interface{}) (map[string]interface{}, error) {
	reqStruct, err := structpb.NewStruct(reqData)
	if err != nil {
		return nil, fmt.Errorf("error convirtiendo datos a Struct para Simplificados: %v", err)
	}

	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("error conectando al host de Simplificados %s: %v", host, err)
	}
	defer conn.Close()

	client := pb.NewSIMPLIFICADOSClient(conn)

	ctx := context.Background()

	res, err := client.SIMPLIFICADOS(ctx, reqStruct)
	if err != nil {
		return nil, fmt.Errorf("error en llamada gRPC a Simplificados: %v", err)
	}

	if res != nil {
		return res.AsMap(), nil
	}

	return nil, nil
}

func ProcessFracciones(host string, reqData map[string]interface{}) (map[string]interface{}, error) {
	reqStruct, err := structpb.NewStruct(reqData)
	if err != nil {
		return nil, fmt.Errorf("error convirtiendo datos a Struct para Fracciones: %v", err)
	}

	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("error conectando al host de Fracciones %s: %v", host, err)
	}
	defer conn.Close()

	client := pb.NewFraccionesServiceClient(conn)

	ctx := context.Background()

	res, err := client.Fracciones(ctx, reqStruct)
	if err != nil {
		return nil, fmt.Errorf("error en llamada gRPC a Fracciones: %v", err)
	}

	if res != nil {
		return res.AsMap(), nil
	}

	return nil, nil
}
