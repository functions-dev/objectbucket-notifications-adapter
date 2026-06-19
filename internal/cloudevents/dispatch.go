package cloudevents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/uuid"
)

type recordsPayload struct {
	Records []json.RawMessage `json:"Records"`
}

func DispatchEvent(ctx context.Context, targetURI string, bucketName string, records []json.RawMessage) error {
	payload := recordsPayload{Records: records}

	e := cloudevents.NewEvent()
	e.SetType("com.noobaa.s3.notification")
	e.SetSource("noobaa/" + bucketName)
	e.SetID(uuid.New().String())
	e.SetTime(time.Now())
	if err := e.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		return fmt.Errorf("setting event data: %w", err)
	}

	c, err := cloudevents.NewClientHTTP(http.WithTarget(targetURI))
	if err != nil {
		return fmt.Errorf("creating cloudevent client for %s: %w", targetURI, err)
	}

	ctx = cloudevents.WithEncodingStructured(ctx)
	result := c.Send(ctx, e)
	if cloudevents.IsUndelivered(result) {
		return fmt.Errorf("failed to send CloudEvent to %s: %v", targetURI, result)
	}
	return nil
}
