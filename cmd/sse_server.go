package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"github.com/spf13/viper"

	"github.com/gin-gonic/gin"
	"irelia/internal/utils/sse"
)

func startSSE() {
	r := gin.Default()
	r.GET("/sse/timeout", SSETimeoutStream)

	sseAddr := fmt.Sprintf(":%s", viper.GetString("server.sseport"))

	// Start the SSE server on a different port or route
    go func() {
		if err := r.Run(sseAddr); err != nil {
            fmt.Printf("Failed to start SSE server: %v\n", err)
        }
    }()
}

func SSETimeoutStream(c *gin.Context) {
    userID := c.Query("user_id") // Or get from auth/session
    if userID == "" {
        c.JSON(400, gin.H{"error": "user_id is required"})
        return
    }

    // Convert userID to uint64
    uid, err := strconv.ParseUint(userID, 10, 64)
    if err != nil {
        c.JSON(400, gin.H{"error": "invalid user_id format"})
        return
    }

    // Set SSE headers
    c.Writer.Header().Set("Content-Type", "text/event-stream")
    c.Writer.Header().Set("Cache-Control", "no-cache")
    c.Writer.Header().Set("Connection", "keep-alive")
    c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
    c.Writer.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

    // Create a channel for this user if not exists
    ch := make(chan map[string]interface{}, 10)
    sse.RegisterChannel(uid, ch)

    // Send initial connection confirmation
    initialMsg := map[string]interface{}{
        "type":      "connection_established",
        "userID":    uid,
        "timestamp": time.Now().Unix(),
    }
    
    if jsonData, err := json.Marshal(initialMsg); err == nil {
        fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
        c.Writer.Flush()
    }

    // Keep connection alive and listen for messages
    clientGone := make(chan bool)
    go func() {
        <-c.Request.Context().Done()
        clientGone <- true
    }()

    // Heartbeat ticker to keep connection alive
    heartbeat := time.NewTicker(60 * time.Second)
    defer heartbeat.Stop()

    for {
        select {
        case <-clientGone:
            // Client disconnected, clean up
            sse.UnregisterChannel(uid)
            return
            
        case <-heartbeat.C:
            // Send heartbeat
            heartbeatMsg := map[string]interface{}{
                "type":      "heartbeat",
                "timestamp": time.Now().Unix(),
            }
            if jsonData, err := json.Marshal(heartbeatMsg); err == nil {
                fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
                c.Writer.Flush()
            }
            
        case notification := <-ch:
            // Send notification to client
            if jsonData, err := json.Marshal(notification); err == nil {
                fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
                c.Writer.Flush()
            }
        }
    }
}