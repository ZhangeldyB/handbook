# Provider Adapter Template

Guide: [gRPC and HTTP Clients](../layers/infrastructure/grpc-clients.md). Rules: [adapter-rules.md](../.agents/rules/adapter-rules.md).

`internal/<context>/adapters/clients/<provider>.go`:

```go
// <Provider> implements ports.<Capability> over the provider's HTTP API.
type <Provider> struct {
	http    *http.Client // shared, with explicit Timeout and transport settings
	baseURL string
	apiKey  string // never logged
}

var _ ports.<Capability> = (*<Provider>)(nil)

type <Provider>Deps struct {
	HTTP    *http.Client
	BaseURL string
	APIKey  string
}

func New<Provider>(d <Provider>Deps) (*<Provider>, error) {
	var errs []error
	errs = append(errs, deps.Require("HTTP", d.HTTP))
	if d.BaseURL == "" || d.APIKey == "" {
		errs = append(errs, errors.New("base URL and API key are required"))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("new <provider> client: %w", err)
	}
	return &<Provider>{http: d.HTTP, baseURL: d.BaseURL, apiKey: d.APIKey}, nil
}

func (p *<Provider>) <Verb>(ctx context.Context, tenantID string, in ports.<Input>, idempotencyKey string) (ports.<Result>, error) {
	body, err := json.Marshal(toProviderRequest(in))
	if err != nil {
		return ports.<Result>{}, fmt.Errorf("encode <verb> request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/<resource>", bytes.NewReader(body))
	if err != nil {
		return ports.<Result>{}, fmt.Errorf("build <verb> request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Idempotency-Key", idempotencyKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		// Network error or timeout: the provider may have applied it.
		return ports.<Result>{}, fmt.Errorf("<verb>: %w: %w", ports.ErrOutcomeUnknown, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return decodeResult(io.LimitReader(resp.Body, 1<<20))
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return ports.<Result>{}, fmt.Errorf("<verb>: status %d: %w", resp.StatusCode, ports.ErrTemporarilyUnavailable)
	default:
		return ports.<Result>{}, fmt.Errorf("<verb>: status %d: %w", resp.StatusCode, ports.ErrRejected)
	}
}
```

Classification comes from status codes and typed errors only; provider error bodies are never returned to your own clients or logged in full.

Tests with `httptest.Server`: success mapping; 4xx → rejected; 429 and 5xx → temporarily unavailable; a handler that sleeps past the client timeout → outcome unknown; the idempotency key header is sent; the API key never appears in errors.
