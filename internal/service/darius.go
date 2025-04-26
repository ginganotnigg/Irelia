package service

import (
    "bytes"
    "context"
    "io"
    "net/http"
    "github.com/spf13/viper"
    "go.uber.org/zap"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/encoding/protojson"

    pb "irelia/api"
)

// DariusClient implements the DariusClient interface using HTTP
type DariusClient struct {
    client *http.Client
    logger *zap.Logger
}

// NewDariusClient creates a new Darius HTTP client
func NewDariusClient(logger *zap.Logger) *DariusClient {
    return &DariusClient{
        client: &http.Client{},
        logger: logger,
    }
}

// Generate sends a REST API request to the Darius service to generate questions
func (d *DariusClient) Generate(ctx context.Context, req *pb.NextQuestionRequest) (*pb.NextQuestionResponse, error) {
    dariusURL := viper.GetString("darius.genurl")

    // Marshal the Protobuf request to JSON
    payloadBytes, err := protojson.Marshal(req)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to marshal request: %v", err)
    }

    // Create the HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, "POST", dariusURL, bytes.NewBuffer(payloadBytes))
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to create HTTP request: %v", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    // Send the HTTP request
    resp, err := d.client.Do(httpReq)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to send HTTP request: %v", err)
    }
    defer resp.Body.Close()

    // Read the response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to read response body: %v", err)
    }

    // Check for non-200 status codes
    if resp.StatusCode != http.StatusOK {
        return nil, status.Errorf(codes.Internal, "Darius service returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
    }

    // Unmarshal the JSON response into a Protobuf message
    var dariusResp pb.NextQuestionResponse
    if err := protojson.Unmarshal(body, &dariusResp); err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to unmarshal response JSON: %v", err)
    }

    return &dariusResp, nil
}

// Score sends a REST API request to the Darius service to score answers
func (d *DariusClient) Score(ctx context.Context, req *pb.ScoreInterviewRequest) (*pb.ScoreInterviewResponse, error) {
    dariusURL := viper.GetString("darius.scrurl")

    // Marshal the Protobuf request to JSON
    payloadBytes, err := protojson.Marshal(req)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to marshal request: %v", err)
    }

    // Create the HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, "POST", dariusURL, bytes.NewBuffer(payloadBytes))
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to create HTTP request: %v", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    // Send the HTTP request
    resp, err := d.client.Do(httpReq)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to send HTTP request: %v", err)
    }
    defer resp.Body.Close()

    // Read the response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to read response body: %v", err)
    }

    // Check for non-200 status codes
    if resp.StatusCode != http.StatusOK {
        return nil, status.Errorf(codes.Internal, "Darius service returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
    }

    // Unmarshal the JSON response into a Protobuf message
    var dariusResp pb.ScoreInterviewResponse
    if err := protojson.Unmarshal(body, &dariusResp); err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to unmarshal response JSON: %v", err)
    }

    return &dariusResp, nil
}