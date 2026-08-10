package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/cyverse-de/messaging/v12"
	amqp "github.com/rabbitmq/amqp091-go"
)

const emailRequestsQueueName = "email_requests"

// emailRequestsPrefetchCount bounds how many unacked deliveries the broker will hand this
// consumer at once.
const emailRequestsPrefetchCount = 100

// AMQPSettings contains the settings needed to consume email requests from AMQP.
type AMQPSettings struct {
	URI          string
	ExchangeName string
	ExchangeType string
}

// AMQPListener consumes email requests from AMQP and processes them in-process.
type AMQPListener struct {
	client    *messaging.Client
	settings  *AMQPSettings
	processor *EmailProcessor
	inFlight  atomic.Int64
}

// NewAMQPListener creates a new AMQP listener with reconnection enabled.
func NewAMQPListener(processor *EmailProcessor, settings *AMQPSettings) (*AMQPListener, error) {
	client, err := messaging.NewClient(settings.URI, true)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to AMQP; this usually means the broker is down or the URI is wrong: %w", err)
	}
	return &AMQPListener{
		client:    client,
		settings:  settings,
		processor: processor,
	}, nil
}

// Listen starts consuming email requests from the durable email_requests queue.
func (l *AMQPListener) Listen() {
	go l.client.Listen()
	l.client.AddConsumer(
		l.settings.ExchangeName,
		l.settings.ExchangeType,
		emailRequestsQueueName,
		messaging.EmailRequestPublishingKey,
		l.handleMessage,
		emailRequestsPrefetchCount,
	)
}

// Drain waits up to timeout for in-flight deliveries to finish processing and ack, so
// emails that were already sent aren't requeued (and re-sent) when the connection closes.
func (l *AMQPListener) Drain(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for l.inFlight.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if n := l.inFlight.Load(); n > 0 {
		log.Warnf("%d email requests still in flight after the %s drain window; their deliveries will be requeued and may be sent twice", n, timeout)
	}
}

// Close shuts down the AMQP connection; unacknowledged deliveries are requeued by the broker.
func (l *AMQPListener) Close() {
	l.client.Close()
}

// handleMessage processes a single delivery. Failures are logged and the message is acked
// anyway: a failed request would almost certainly fail again on redelivery, and dropping it
// matches the behavior of the retired email-requests service.
func (l *AMQPListener) handleMessage(ctx context.Context, delivery amqp.Delivery) {
	l.inFlight.Add(1)
	defer l.inFlight.Add(-1)
	alog := log.WithContext(ctx).WithField("transport", "amqp")
	if err := l.processor.Process(ctx, delivery.Body); err != nil {
		alog.Errorf("failed to process email request; the message will be dropped: %s", err)
	}
	if err := delivery.Ack(false); err != nil {
		alog.Errorf("failed to ack email request message: %s", err)
	}
}

// verifyAMQPTopology declares the exchange, queue, and binding on a short-lived connection,
// mirroring the messaging library's own declarations. The library silently discards
// declaration errors during consumer setup, so this is the only place a broker/config
// mismatch (e.g. a wrong exchange type) surfaces as an error instead of a silent no-consumer.
func verifyAMQPTopology(settings *AMQPSettings) error {
	conn, err := amqp.Dial(settings.URI)
	if err != nil {
		return fmt.Errorf("unable to connect to the AMQP broker: %w", err)
	}
	defer conn.Close() // nolint:errcheck

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("unable to open an AMQP channel: %w", err)
	}
	defer ch.Close() // nolint:errcheck

	if err := ch.ExchangeDeclare(settings.ExchangeName, settings.ExchangeType, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring exchange %q (type %q) failed; this usually means amqp.exchange.type doesn't match the existing exchange: %w",
			settings.ExchangeName, settings.ExchangeType, err)
	}
	if _, err := ch.QueueDeclare(emailRequestsQueueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring queue %q failed; this usually means the existing queue was declared with different arguments: %w",
			emailRequestsQueueName, err)
	}
	if err := ch.QueueBind(emailRequestsQueueName, messaging.EmailRequestPublishingKey, settings.ExchangeName, false, nil); err != nil {
		return fmt.Errorf("binding queue %q to exchange %q failed: %w", emailRequestsQueueName, settings.ExchangeName, err)
	}
	return nil
}

// startAMQPConsumer verifies the AMQP topology (retrying with backoff so a broker outage
// never blocks or kills HTTP delivery), then starts the consumer. It returns nil if ctx is
// canceled before the broker becomes reachable.
func startAMQPConsumer(ctx context.Context, processor *EmailProcessor, settings *AMQPSettings) *AMQPListener {
	backoff := time.Second
	for {
		err := verifyAMQPTopology(settings)
		if err == nil {
			break
		}
		log.Errorf("AMQP consumer setup failed, retrying in %s (HTTP email delivery is unaffected): %s", backoff, err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}

	listener, err := NewAMQPListener(processor, settings)
	if err != nil {
		log.Errorf("unable to create the AMQP listener; AMQP email requests will not be consumed: %s", err)
		return nil
	}
	listener.Listen()
	log.Infof("consuming AMQP queue %s", emailRequestsQueueName)
	return listener
}
