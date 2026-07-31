package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/service"
	"github.com/google/uuid"
	natslib "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type wsEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type JetStreamEventPublisher struct {
	nc  *natslib.Conn
	js  jetstream.JetStream
	log *slog.Logger
}

func NewJetStreamEventPublisher(ctx context.Context, natsURL string, log *slog.Logger) (*JetStreamEventPublisher, error) {
	nc, err := natslib.Connect(natsURL,
		natslib.Name("chinwag-chat"),
		natslib.ReconnectWait(2*time.Second),
		natslib.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream new: %w", err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "CHAT_EVENTS",
		Description: "Chat service domain events for cross-instance delivery",
		Subjects:    []string{"chat.room.>", "chat.system"},
		Retention:   jetstream.InterestPolicy,
		Storage:     jetstream.FileStorage,
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream stream setup: %w", err)
	}

	return &JetStreamEventPublisher{nc: nc, js: js, log: log}, nil
}

func (p *JetStreamEventPublisher) PublishRaw(ctx context.Context, subject string, data []byte) error {
	p.log.Info("publishing raw event",
		"subject", subject,
	)

	_, err := p.js.Publish(ctx, subject, data)
	if err != nil {
		p.log.Error("failed to publish raw event",
			"subject", subject,
			"error", err,
		)
		return fmt.Errorf("jetstream publish raw: %w", err)
	}

	p.log.Info("raw event published",
		"subject", subject,
	)

	return nil
}

func (p *JetStreamEventPublisher) Publish(roomId uuid.UUID, event service.Event) error {
	data, err := json.Marshal(wsEvent{Type: event.Type, Data: event.Data})
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}

	subject := fmt.Sprintf("chat.room.%s", roomId.String())
	p.log.Info("publishing event",
		"subject", subject,
		"event_type", event.Type,
		"room_id", roomId.String(),
	)

	_, err = p.js.Publish(context.Background(), subject, data)
	if err != nil {
		p.log.Error("failed to publish event",
			"subject", subject,
			"event_type", event.Type,
			"room_id", roomId.String(),
			"error", err,
		)
		return fmt.Errorf("jetstream publish: %w", err)
	}

	p.log.Info("event published",
		"subject", subject,
		"event_type", event.Type,
		"room_id", roomId.String(),
	)

	return nil
}

func (p *JetStreamEventPublisher) Consume(ctx context.Context, consumerName string, onEvent func(roomId uuid.UUID, data []byte)) error {
	stream, err := p.js.Stream(ctx, "CHAT_EVENTS")
	if err != nil {
		return fmt.Errorf("jetstream stream lookup: %w", err)
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          consumerName,
		Description:   "Deliver chat events to this service instance",
		FilterSubject: "chat.room.>",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
	})
	if err != nil {
		return fmt.Errorf("jetstream consumer: %w", err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		if !strings.HasPrefix(msg.Subject(), "chat.room.") {
			p.log.Warn("unexpected subject format, skipping", "subject", msg.Subject())
			msg.Ack()
			return
		}

		roomIdStr := msg.Subject()[strings.LastIndex(msg.Subject(), ".")+1:]
		roomId, err := uuid.Parse(roomIdStr)
		if err != nil {
			p.log.Warn("invalid room id in subject", "subject", msg.Subject(), "room_id", roomIdStr)
			msg.Ack()
			return
		}

		var ev wsEvent
		json.Unmarshal(msg.Data(), &ev)

		p.log.Info("consuming event",
			"subject", msg.Subject(),
			"room_id", roomId.String(),
			"event_type", ev.Type,
		)

		onEvent(roomId, msg.Data())
		msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("jetstream consume start: %w", err)
	}

	go func() {
		<-ctx.Done()
		cc.Stop()
	}()

	return nil
}

func (p *JetStreamEventPublisher) Close() {
	p.nc.Close()
}
