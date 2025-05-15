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

// KarmaClient implements the KarmaClient interface using HTTP
type KarmaClient struct {
    client *http.Client
    logger *zap.Logger
}

// NewKarmaClient creates a new Karma HTTP client
func NewKarmaClient(logger *zap.Logger) *KarmaClient {
    return &KarmaClient{
        client: &http.Client{},
        logger: logger,
    }
}

// LipSync sends a REST API request to the Karma service
func (k *KarmaClient) LipSync(ctx context.Context, req *pb.LipSyncRequest) (*pb.LipSyncResponse, error) {
    karmaURL := viper.GetString("karma.genurl")

    // Marshal the Protobuf request to JSON
    payloadBytes, err := protojson.Marshal(req)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to marshal request: %v", err)
    }

    // Create the HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, "POST", karmaURL, bytes.NewBuffer(payloadBytes))
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to create HTTP request: %v", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    // Send the HTTP request
    resp, err := k.client.Do(httpReq)
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
        return nil, status.Errorf(codes.Internal, "Karma service returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
    }

    // Unmarshal the JSON response into a Protobuf message
    var karmaResp pb.LipSyncResponse
    if err := protojson.Unmarshal(body, &karmaResp); err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to unmarshal response JSON: %v", err)
    }

    return &karmaResp, nil
}

// LipSync sends a REST API request to the Karma service
func (k *KarmaClient) Score(ctx context.Context, req *pb.ScoreFluencyRequest) (*pb.ScoreFluencyResponse, error) {
    karmaURL := viper.GetString("karma.scrurl")

    // Marshal the Protobuf request to JSON
    payloadBytes, err := protojson.Marshal(req)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to marshal request: %v", err)
    }

    // Create the HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, "POST", karmaURL, bytes.NewBuffer(payloadBytes))
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to create HTTP request: %v", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    // Send the HTTP request
    resp, err := k.client.Do(httpReq)
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
        return nil, status.Errorf(codes.Internal, "Karma service returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
    }

    // Unmarshal the JSON response into a Protobuf message
    var karmaResp pb.ScoreFluencyResponse
    if err := protojson.Unmarshal(body, &karmaResp); err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to unmarshal response JSON: %v", err)
    }

    return &karmaResp, nil
}