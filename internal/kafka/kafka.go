package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/user/fx-settlement-engine/internal/domain"
)

type EventProducer struct {
	writer *kafka.Writer
}

func NewEventProducer(brokers []string, topic string) *EventProducer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return &EventProducer{writer: writer}
}

func (p *EventProducer) PublishAuditEvent(ctx context.Context, event *domain.AuditEvent) error {
	if p.writer == nil {
		return nil
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(event.TransactionID),
		Value: payload,
		Time:  time.Now(),
	}

	err = p.writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Printf("[Kafka Producer] Warning: failed to write message to Kafka: %v", err)
		return nil // Non-blocking for core settlement flow
	}

	log.Printf("[Kafka Producer] Published audit event for transaction %s", event.TransactionID)
	return nil
}

func (p *EventProducer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}

type AuditConsumer struct {
	reader *kafka.Reader
}

func NewAuditConsumer(brokers []string, topic, groupID string) *AuditConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10 * 1024,      // 10KB
		MaxBytes: 10 * 1024 * 1024, // 10MB
	})
	return &AuditConsumer{reader: reader}
}

func (c *AuditConsumer) StartListening(ctx context.Context) {
	log.Println("[Kafka Consumer] Audit listener started...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Kafka Consumer] Shutting down audit listener...")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Kafka Consumer] Error reading message: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			var event domain.AuditEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[Kafka Consumer] Error unmarshalling message: %v", err)
				continue
			}

			log.Printf("[AUDIT LOG CONSUMER] Event Received: ID=%s | Type=%s | TxID=%s | Sender=%s -> Receiver=%s | Amount=%.2f %s -> %.2f %s | Status=%s",
				event.EventID, event.EventType, event.TransactionID, event.SenderID, event.ReceiverID,
				event.FromAmount, event.FromCurrency, event.ToAmount, event.ToCurrency, event.Status,
			)
		}
	}
}

func (c *AuditConsumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}
