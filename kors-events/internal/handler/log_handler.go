package handler

import (
	"context"
	"encoding/json"
	"log"
)

type LogHandler struct{}

func (h *LogHandler) Handle(ctx context.Context, subject string, payload []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("LogHandler: failed to decode payload for %s: %v", subject, err)
		return err
	}
	
	resourceID := "unknown"
	if res, ok := data["resource"].(map[string]interface{}); ok {
		if id, ok := res["id"].(string); ok {
			resourceID = id
		}
	} else if id, ok := data["resource_id"].(string); ok {
		resourceID = id
	}

	log.Printf("LogHandler: Event received: subject=%s, resource_id=%s", subject, resourceID)
	return nil
}