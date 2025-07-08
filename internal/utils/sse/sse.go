package sse

import (
    "sync"
)

var sseChannels sync.Map // key: uint64, value: chan map[string]interface{}

func RegisterChannel(userID uint64, ch chan map[string]interface{}) {
    sseChannels.Store(userID, ch)
}

func UnregisterChannel(userID uint64) {
    sseChannels.Delete(userID)
}

func SendToUser(userID uint64, notification map[string]interface{}) bool {
    if chVal, ok := sseChannels.Load(userID); ok {
        if ch, ok := chVal.(chan map[string]interface{}); ok {
            select {
            case ch <- notification:
                return true
            default:
                return false
            }
        }
    }
    return false
}