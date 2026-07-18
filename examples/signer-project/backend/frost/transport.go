package frostsandbox

import (
	"context"
	"fmt"
	"sync"

	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
)

// protocolResult is emitted only after one participant's handler closes and
// exposes its final result. The transport below is intentionally in-memory.
type protocolResult struct {
	id    party.ID
	value any
	err   error
}

func runHandlers(ctx context.Context, handlers map[party.ID]*protocol.MultiHandler) (map[party.ID]any, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	inboxes := make(map[party.ID]chan *protocol.Message, len(handlers))
	for id := range handlers {
		inboxes[id] = make(chan *protocol.Message, 4096)
	}

	completed := make(chan protocolResult, len(handlers))
	var wg sync.WaitGroup
	for id, handler := range handlers {
		id := id
		handler := handler
		wg.Add(1)
		go func() {
			defer wg.Done()
			driveHandler(ctx, id, handler, inboxes, completed)
		}()
	}

	results := make(map[party.ID]any, len(handlers))
	for range handlers {
		select {
		case result := <-completed:
			if result.err != nil {
				cancel()
				wg.Wait()
				return nil, fmt.Errorf("participant %q: %w", result.id, result.err)
			}
			results[result.id] = result.value
		case <-ctx.Done():
			cancel()
			wg.Wait()
			return nil, ctx.Err()
		}
	}
	wg.Wait()
	return results, nil
}

func driveHandler(
	ctx context.Context,
	id party.ID,
	handler *protocol.MultiHandler,
	inboxes map[party.ID]chan *protocol.Message,
	completed chan<- protocolResult,
) {
	for {
		select {
		case message, ok := <-handler.Listen():
			if !ok {
				value, err := handler.Result()
				select {
				case completed <- protocolResult{id: id, value: value, err: err}:
				case <-ctx.Done():
				}
				return
			}
			for recipient, inbox := range inboxes {
				if !message.IsFor(recipient) {
					continue
				}
				select {
				case inbox <- cloneMessage(message):
				case <-ctx.Done():
					return
				}
			}
		case message := <-inboxes[id]:
			handler.Accept(message)
		case <-ctx.Done():
			return
		}
	}
}

func cloneMessage(message *protocol.Message) *protocol.Message {
	copy := *message
	copy.SSID = append([]byte(nil), message.SSID...)
	copy.Data = append([]byte(nil), message.Data...)
	copy.BroadcastVerification = append([]byte(nil), message.BroadcastVerification...)
	return &copy
}
