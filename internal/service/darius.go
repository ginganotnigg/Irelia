package service

import (
    "bytes"
    "context"
    "encoding/json"
    "io"
    "net/http"

    "github.com/spf13/viper"
    "go.uber.org/zap"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    pb "irelia/api"
    repo "irelia/internal/repo"
)

// DariusService handles communication with the Darius service
type DariusService struct {
    dariusClient DariusClient
    interviewRepo repo.SQLInterviewRepository
    logger        *zap.Logger
}

// NewDariusService creates a new DariusService instance
func NewDariusService(dariusClient DariusClient, repo repo.SQLInterviewRepository, logger *zap.Logger) *DariusService {
    return &DariusService{
        dariusClient:  dariusClient,
        interviewRepo: repo,
        logger:        logger,
    }
}

// DariusClient defines the interface for the Darius service client
type DariusClient interface {
    CallDarius(ctx context.Context, payload map[string]interface{}) (*pb.NextQuestionResponse, error)
}

// DariusHTTPClient implements the DariusClient interface using HTTP
type DariusHTTPClient struct {
    client *http.Client
}

// NewDariusHTTPClient creates a new Darius HTTP client
func NewDariusHTTPClient() *DariusHTTPClient {
    return &DariusHTTPClient{
        client: &http.Client{},
    }
}

// CallDarius sends a REST API request to the Darius service
func (d *DariusHTTPClient) CallDarius(ctx context.Context, payload map[string]interface{}) (*pb.NextQuestionResponse, error) {
    dariusURL := viper.GetString("darius.url")

    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to marshal payload: %v", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST", dariusURL, bytes.NewBuffer(payloadBytes))
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to create HTTP request: %v", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := d.client.Do(httpReq)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to send HTTP request: %v", err)
	}
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to read response body: %v", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, status.Errorf(codes.Internal, "Darius service returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
    }

    var dariusResp struct {
        Questions []string `json:"questions"`
    }
    if err := json.Unmarshal(body, &dariusResp); err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to unmarshal response JSON: %v", err)
    }

    return &pb.NextQuestionResponse{
        Questions: dariusResp.Questions,
    }, nil
}