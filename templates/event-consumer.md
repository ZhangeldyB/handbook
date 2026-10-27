# Event Consumer Template

Working version: [payment_captured.go](../examples/orders/internal/orders/adapters/eventconsumer/payment_captured.go) with [tests](../examples/orders/internal/orders/adapters/eventconsumer/payment_captured_test.go). Pattern: [Event Consumer](../patterns/event-consumer.md).

`internal/<context>/adapters/eventconsumer/<topic_name>.go`:

```go
const Topic<Event> = "<producer>.<event>.v1"

// <event>V1 is the foreign wire contract; it never crosses inward.
type <event>V1 struct {
	<Entity>ID string `json:"<entity>_id"`
	// ...
}

type <command>Command interface {
	Execute(ctx context.Context, req usecases.<Command>Request) error
}

type <Event> struct{ cmd <command>Command }

var _ messaging.Handler = (*<Event>)(nil)

func New<Event>(cmd <command>Command) (*<Event>, error) {
	if err := deps.Require("<Command>", cmd); err != nil {
		return nil, fmt.Errorf("new <event> handler: %w", err)
	}
	return &<Event>{cmd: cmd}, nil
}

func (h *<Event>) Handle(ctx context.Context, msg messaging.Message) error {
	var payload <event>V1
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s %q: %w: %w", msg.Topic, msg.ID, messaging.ErrPermanent, err)
	}
	if msg.ID == "" || msg.TenantID == "" || payload.<Entity>ID == "" {
		return fmt.Errorf("validate %s %q: missing identity: %w", msg.Topic, msg.ID, messaging.ErrPermanent)
	}
	err := h.cmd.Execute(ctx, usecases.<Command>Request{
		TenantID: msg.TenantID, <Entity>ID: payload.<Entity>ID, EventID: msg.ID,
	})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ports.ErrNotFound), errors.Is(err, domain.ErrInvalidTransition):
		return fmt.Errorf("handle %s %q: %w: %w", msg.Topic, msg.ID, messaging.ErrPermanent, err)
	default:
		return fmt.Errorf("handle %s %q: %w", msg.Topic, msg.ID, err) // redeliver
	}
}
```

The command saves with `ports.IdempotencyRecord{Scope: "consume:" + Topic<Event>, Key: req.EventID}` and treats `ports.ErrDuplicate` as success.

Register the handler for its topic in the composition root; a subscription without a handler is a startup error.

Tests: mapping to the command; malformed and incomplete messages are permanent and never call the command; permanent versus retried command errors; duplicate delivery applies once.
