package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/companyofcreators/order-service/internal/config"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type Producer struct {
	writer *kafka.Writer
	cfg    *config.Config
}

type OrderCreatedEvent struct {
	OrderID    string  `json:"order_id"`
	CustomerID string  `json:"customer_id"`
	CategoryID string  `json:"category_id"`
	Status     string  `json:"status"`
	Price      float64 `json:"price"`
	Title      string  `json:"title"`
	Timestamp  string  `json:"timestamp"`
}

type OrderAssignedEvent struct {
	OrderID    string `json:"order_id"`
	MasterID   string `json:"master_id"`
	CustomerID string `json:"customer_id"`
	Timestamp  string `json:"timestamp"`
}

type OrderCompletedEvent struct {
	OrderID    string  `json:"order_id"`
	CustomerID string  `json:"customer_id"`
	MasterID   string  `json:"master_id"`
	FinalPrice float64 `json:"final_price"`
	Timestamp  string  `json:"timestamp"`
}

type OrderCancelledEvent struct {
	OrderID     string `json:"order_id"`
	CancelledBy string `json:"cancelled_by"`
	Reason      string `json:"reason"`
	Timestamp   string `json:"timestamp"`
}

type ReviewCreatedEvent struct {
	ReviewID   string `json:"review_id"`
	OrderID    string `json:"order_id"`
	FromUserID string `json:"from_user_id"`
	ToUserID   string `json:"to_user_id"`
	Rating     int    `json:"rating"`
	Timestamp  string `json:"timestamp"`
}

type ComplaintCreatedEvent struct {
	ComplaintID string `json:"complaint_id"`
	ReporterID  string `json:"reporter_id"`
	TargetID    string `json:"target_id"`
	OrderID     string `json:"order_id,omitempty"`
	Message     string `json:"message"`
	Timestamp   string `json:"timestamp"`
}

func NewProducer(cfg *config.Config) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokersList()...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
		BatchSize:    100,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  5 * time.Second,
	}

	return &Producer{
		writer: writer,
		cfg:    cfg,
	}
}

func (p *Producer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}

func (p *Producer) PublishOrderCreated(ctx context.Context, event OrderCreatedEvent) error {
	return p.publish(ctx, p.cfg.KafkaTopicOrderCreated, event.OrderID, event)
}

func (p *Producer) PublishOrderAssigned(ctx context.Context, event OrderAssignedEvent) error {
	return p.publish(ctx, p.cfg.KafkaTopicOrderAssigned, event.OrderID, event)
}

func (p *Producer) PublishOrderCompleted(ctx context.Context, event OrderCompletedEvent) error {
	return p.publish(ctx, p.cfg.KafkaTopicOrderCompleted, event.OrderID, event)
}

func (p *Producer) PublishOrderCancelled(ctx context.Context, event OrderCancelledEvent) error {
	return p.publish(ctx, p.cfg.KafkaTopicOrderCancelled, event.OrderID, event)
}

func (p *Producer) PublishReviewCreated(ctx context.Context, event ReviewCreatedEvent) error {
	return p.publish(ctx, p.cfg.KafkaTopicReviewCreated, event.ReviewID, event)
}

func (p *Producer) PublishComplaintCreated(ctx context.Context, event ComplaintCreatedEvent) error {
	return p.publish(ctx, p.cfg.KafkaTopicComplaintCreated, event.ComplaintID, event)
}

func (p *Producer) publish(ctx context.Context, topic string, key string, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to marshal kafka event", "error", err.Error(), "topic", topic)
		return fmt.Errorf("marshal event for topic %s: %w", topic, err)
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to publish kafka message",
			"error", err.Error(),
			"topic", topic,
			"key", key,
		)
		return fmt.Errorf("publish to topic %s: %w", topic, err)
	}

	pkg.Logger.DebugContext(ctx, "kafka message published",
		"topic", topic,
		"key", key,
	)

	return nil
}
