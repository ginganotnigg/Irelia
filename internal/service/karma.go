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
)

// KarmaService handles communication with the Karma service
type KarmaService struct {
    karmaClient KarmaClient
    logger      *zap.Logger
}

// NewKarmaService creates a new KarmaService instance
func NewKarmaService(karmaClient KarmaClient, logger *zap.Logger) *KarmaService {
    return &KarmaService{
        karmaClient: karmaClient,
        logger:      logger,
    }
}

// KarmaClient defines the interface for the Karma service client
type KarmaClient interface {
    CallKarma(ctx context.Context, payload map[string]interface{}) (*pb.LipSyncResponse, error)
}

// KarmaHTTPClient implements the KarmaClient interface using HTTP
type KarmaHTTPClient struct {
    client *http.Client
}

type KarmaResponse struct {
    Audio   string `json:"audio"`
    Lipsync struct {
        Metadata struct {
            SoundFile string  `json:"soundFile"`
            Duration float32 `json:"duration"`
        } `json:"metadata"`
        MouthCues []struct {
            Start float32 `json:"start"`
            End   float32 `json:"end"`
            Value string  `json:"value"`
        } `json:"mouthCues"`
        Error string `json:"error,omitempty"`
    } `json:"lipsync"`
}

// NewKarmaHTTPClient creates a new Karma HTTP client
func NewKarmaHTTPClient() *KarmaHTTPClient {
    return &KarmaHTTPClient{
        client: &http.Client{},
    }
}

// CallKarma sends a REST API request to the Karma service
func (k *KarmaHTTPClient) CallKarma(ctx context.Context, payload map[string]interface{}) (*pb.LipSyncResponse, error) {
    karmaURL := viper.GetString("karma.url")

    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to marshal payload: %v", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST", karmaURL, bytes.NewBuffer(payloadBytes))
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to create HTTP request: %v", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := k.client.Do(httpReq)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to send HTTP request: %v", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to read response body: %v", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, status.Errorf(codes.Internal, "Karma service returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
    }

    var karmaResp KarmaResponse
    if err := json.Unmarshal(body, &karmaResp); err != nil {
        return nil, status.Errorf(codes.Internal, "Failed to unmarshal response JSON: %v", err)
    }

    // Convert to protobuf response
    lipSyncData := &pb.LipSyncData{
        Metadata: &pb.LipSyncMetadata{
            SoundFile: karmaResp.Lipsync.Metadata.SoundFile,
            Duration:  karmaResp.Lipsync.Metadata.Duration,
        },
        MouthCues: make([]*pb.MouthCue, len(karmaResp.Lipsync.MouthCues)),
    }

    for i, cue := range karmaResp.Lipsync.MouthCues {
        lipSyncData.MouthCues[i] = &pb.MouthCue{
            Start: cue.Start,
            End:   cue.End,
            Value: cue.Value,
        }
    }

    return &pb.LipSyncResponse{
        Audio:   karmaResp.Audio,
        Lipsync: lipSyncData,
    }, nil
}