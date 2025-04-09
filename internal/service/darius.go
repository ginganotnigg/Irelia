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
    CallDariusForGenerate(ctx context.Context, payload map[string]interface{}) (*pb.NextQuestionResponse, error)
    CallDariusForScore(ctx context.Context, payload map[string]interface{}) (*pb.ScoreInterviewResponse, error)
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

// CallDariusForGenerate sends a REST API request to the Darius service to generate questions
func (d *DariusHTTPClient) CallDariusForGenerate(ctx context.Context, payload map[string]interface{}) (*pb.NextQuestionResponse, error) {
    dariusURL := viper.GetString("darius.genurl")

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
        Questions []string `json:"question"`
    }
    if err := json.Unmarshal(body, &dariusResp); err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to unmarshal response JSON: %v", err)
    }

    return &pb.NextQuestionResponse{
        Questions: dariusResp.Questions,
    }, nil
}

// CallDariusForScore sends a REST API request to the Darius service to score answers
func (d *DariusHTTPClient) CallDariusForScore(ctx context.Context, payload map[string]interface{}) (*pb.ScoreInterviewResponse, error) {
    dariusURL := viper.GetString("darius.scrurl")

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
        Submissions []struct {
            Index   int32  `json:"index"`
            Comment string `json:"comment"`
            Score   string `json:"score"`
        } `json:"result"`
        TotalScore struct {
            A int32 `json:"A"`
            B int32 `json:"B"`
            C int32 `json:"C"`
            D int32 `json:"D"`
            F int32 `json:"F"`
        } `json:"totalScore"`
        PositiveFeedback   string `json:"positiveFeedback"`
        ActionableFeedback string `json:"actionableFeedback"`
        FinalComment       string `json:"finalComment"`
    }

    if err := json.Unmarshal(body, &dariusResp); err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to unmarshal response JSON: %v", err)
    }

    // Convert the response to protobuf format
    submissions := make([]*pb.AnswerScore, len(dariusResp.Submissions))
    for i, submission := range dariusResp.Submissions {
        submissions[i] = &pb.AnswerScore{
            Index:   submission.Index,
            Comment: submission.Comment,
            Score:   submission.Score,
        }
    }

    totalScore := &pb.TotalScore{
        A: dariusResp.TotalScore.A,
        B: dariusResp.TotalScore.B,
        C: dariusResp.TotalScore.C,
        D: dariusResp.TotalScore.D,
        F: dariusResp.TotalScore.F,
    }

    return &pb.ScoreInterviewResponse{
        Submissions:        submissions,
        TotalScore:         totalScore,
        PositiveFeedback:   dariusResp.PositiveFeedback,
        ActionableFeedback: dariusResp.ActionableFeedback,
        FinalComment:       dariusResp.FinalComment,
    }, nil
}