package grpcclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	pb "orquestador_p/pb"
)

func ProcessFile(host string, filePath string) (map[string]interface{}, error) {
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewFileProcessorClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
	defer cancel()

	req := &pb.FileRequest{FilePath: filePath}
	res, err := client.ProcessFile(ctx, req)
	if err != nil {
		return nil, err
	}

	if res.Data != nil {
		return res.Data.AsMap(), nil
	}

	return nil, nil
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
