package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/infrai-examples/creator-device-dashboard/internal/infrai"
)

type DeviceUpdate struct {
	AssetID          string `json:"asset_id"`
	SubscriberID     string `json:"subscriber_id"`
	DeviceID         string `json:"device_id"`
	DeliveryStatus   string `json:"delivery_status"`
	ProcessingStatus string `json:"processing_status"`
}

type PublishedUpdate struct {
	Channel string `json:"channel"`
	Event   string `json:"event"`
}

type Publisher interface {
	Publish(context.Context, infrai.PublishInput, string) error
}

type Stream struct {
	publisher Publisher
	channel   string
}

func NewStream(publisher Publisher, channel string) *Stream {
	return &Stream{publisher: publisher, channel: channel}
}

func (s *Stream) Send(ctx context.Context, update DeviceUpdate) (PublishedUpdate, error) {
	if strings.TrimSpace(update.AssetID) == "" || strings.TrimSpace(update.SubscriberID) == "" || strings.TrimSpace(update.DeviceID) == "" {
		return PublishedUpdate{}, fmt.Errorf("asset_id, subscriber_id, and device_id are required")
	}
	event := "content.processing.updated"
	if update.ProcessingStatus == "ready" && update.DeliveryStatus == "delivered" {
		event = "asset.delivery.ready"
	}

	sum := sha256.Sum256([]byte(strings.Join([]string{
		update.AssetID, update.SubscriberID, update.DeviceID, update.DeliveryStatus, update.ProcessingStatus,
	}, "\x00")))
	idempotencyKey := "device-update-" + hex.EncodeToString(sum[:])
	err := s.publisher.Publish(ctx, infrai.PublishInput{
		Channel: s.channel, Event: event, Data: update, AccountID: update.SubscriberID,
	}, idempotencyKey)
	if err != nil {
		return PublishedUpdate{}, err
	}
	return PublishedUpdate{Channel: s.channel, Event: event}, nil
}
