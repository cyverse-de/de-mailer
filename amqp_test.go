package main

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// fakeAcknowledger records acknowledgements for a delivery.
type fakeAcknowledger struct {
	acks, nacks, rejects int
}

func (f *fakeAcknowledger) Ack(_ uint64, _ bool) error {
	f.acks++
	return nil
}

func (f *fakeAcknowledger) Nack(_ uint64, _, _ bool) error {
	f.nacks++
	return nil
}

func (f *fakeAcknowledger) Reject(_ uint64, _ bool) error {
	f.rejects++
	return nil
}

// TestHandleMessage verifies the log-and-ack contract: every delivery is acked exactly
// once whether processing succeeds or fails, and never nacked or rejected.
func TestHandleMessage(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantSent int
	}{
		{
			name:     "valid email request",
			body:     `{"template":"blank","subject":"s","to":"user@example.org","values":{"contents":"x"}}`,
			wantSent: 1,
		},
		{
			name:     "garbage payload",
			body:     `not even json`,
			wantSent: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &fakeSender{}
			listener := &AMQPListener{processor: NewEmailProcessor(sender, testDESettings(), "noreply@example.org")}
			acker := &fakeAcknowledger{}

			listener.handleMessage(context.Background(), amqp.Delivery{
				Acknowledger: acker,
				Body:         []byte(tt.body),
			})

			if acker.acks != 1 || acker.nacks != 0 || acker.rejects != 0 {
				t.Errorf("expected exactly one ack and nothing else, got acks=%d nacks=%d rejects=%d",
					acker.acks, acker.nacks, acker.rejects)
			}
			if len(sender.sent) != tt.wantSent {
				t.Errorf("expected %d sent messages, got %d", tt.wantSent, len(sender.sent))
			}
		})
	}
}
