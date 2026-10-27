# gRPC Handler Template

Supporting packages (compiled and tested): [auth](../examples/grpctransport/auth/auth.go), [grpcerr](../examples/grpctransport/grpcerr/grpcerr.go), [validation](../examples/grpctransport/validation/validation.go). Rules: [handler-rules.md](../.agents/rules/handler-rules.md).

## Files

```text
internal/<context>/adapters/grpc/
├── server.go        # <Entity>Server struct, constructor, Deps
├── <rpc>.go         # one RPC per file when the service is large
├── mapping.go       # proto ↔ application mapping, enum maps
└── errors.go        # the grpcerr rule table for this context
```

## Server and Handler

```go
type <Entity>ServerDeps struct {
	Activate activateCommand
	Queries  *queries.Queries
}

type activateCommand interface {
	Execute(ctx context.Context, req usecases.ActivateRequest) error
}

type <Entity>Server struct {
	<context>v1.Unimplemented<Entity>ServiceServer
	activate activateCommand
	queries  *queries.Queries
}

func New<Entity>Server(d <Entity>ServerDeps) (*<Entity>Server, error) {
	if err := errors.Join(deps.Require("Activate", d.Activate), deps.Require("Queries", d.Queries)); err != nil {
		return nil, fmt.Errorf("new <entity> server: %w", err)
	}
	return &<Entity>Server{activate: d.Activate, queries: d.Queries}, nil
}

func (s *<Entity>Server) Activate<Entity>(ctx context.Context, req *<context>v1.Activate<Entity>Request) (*<context>v1.Activate<Entity>Response, error) {
	var v validation.Violations
	if req.Get<Entity>Id() == "" {
		v.Add("<entity>_id", validation.Required, "<entity>_id is required")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	principal, ok := auth.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if err := s.activate.Execute(ctx, usecases.ActivateRequest{
		TenantID:   principal.TenantID,
		<Entity>ID: req.Get<Entity>Id(),
	}); err != nil {
		return nil, err // mapped by the grpcerr interceptor
	}
	return &<context>v1.Activate<Entity>Response{}, nil
}
```

## Error Rules

```go
// ErrorRules is installed once in the composition root: grpcerr.NewMapper(ErrorRules()...).
func ErrorRules() []grpcerr.Rule {
	return []grpcerr.Rule{
		{Target: ports.ErrNotFound, Code: codes.NotFound, Message: "<entity> not found"},
		{Target: ports.ErrConcurrentModification, Code: codes.Aborted, Message: "<entity> was modified concurrently; retry"},
		{Target: domain.ErrInvalidTransition, Code: codes.FailedPrecondition, Message: "<entity> is not in a valid state for this operation"},
	}
}
```

## Tests

Over `bufconn` with the real interceptor chain: missing principal → `Unauthenticated` and no command call; invalid input → `InvalidArgument` with (field, reason) pairs; tenant passed from the principal; mapping including optional, zero, and unknown enum values; each rule's code.
