package dashboard

import (
	"context"
	"testing"

	"github.com/infrai-examples/creator-device-dashboard/internal/infrai"
)

type recordingPublisher struct {
	input infrai.PublishInput
	key   string
}

func (p *recordingPublisher) Publish(_ context.Context, input infrai.PublishInput, key string) error {
	p.input, p.key = input, key
	return nil
}

func TestStreamSendChoosesDashboardEvent(t *testing.T) {
	tests := []struct {
		name             string
		deliveryStatus   string
		processingStatus string
		wantEvent        string
	}{
		{name: "finished delivery", deliveryStatus: "delivered", processingStatus: "ready", wantEvent: "asset.delivery.ready"},
		{name: "transcode remains visible", deliveryStatus: "pending", processingStatus: "transcoding", wantEvent: "content.processing.updated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &recordingPublisher{}
			stream := NewStream(publisher, "creator-device-status")
			got, err := stream.Send(context.Background(), DeviceUpdate{
				AssetID: "asset-42", SubscriberID: "subscriber-7", DeviceID: "tablet-3",
				DeliveryStatus: tt.deliveryStatus, ProcessingStatus: tt.processingStatus,
			})
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			if got.Event != tt.wantEvent || publisher.input.Event != tt.wantEvent {
				t.Fatalf("event = %q, published = %q, want %q", got.Event, publisher.input.Event, tt.wantEvent)
			}
			if publisher.input.AccountID != "subscriber-7" || publisher.key == "" {
				t.Fatalf("publish boundary missing subscriber or idempotency key")
			}
		})
	}
}
