# Composition Root Template

Guides: [Wiring](../layers/infrastructure/wiring.md), [Configuration](../layers/shared/configuration.md), [Graceful Shutdown](../layers/infrastructure/graceful-shutdown.md). A runnable miniature: [demo/main.go](../examples/orders/cmd/demo/main.go).

`cmd/server/main.go`:

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, stop, os.Getenv); err != nil {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stopSignals func(), getenv func(string) string) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := telemetry.NewLogger(cfg.Log)
	tp, err := telemetry.NewTracerProvider(ctx, cfg.Trace)
	if err != nil {
		return err
	}

	// Infrastructure.
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	publisher, err := broker.NewPublisher(ctx, cfg.Broker)
	if err != nil {
		return fmt.Errorf("connect broker: %w", err)
	}

	// Adapters and application.
	repo, err := persistence.New(db)
	if err != nil {
		return err
	}
	place, err := usecases.NewPlace(usecases.PlaceDeps{Orders: repo, Clock: clock.System{}, IDs: ids.Random{}})
	if err != nil {
		return err
	}
	// ...every other command and query, each with its own Deps

	// Transports and workers.
	mapper := grpcerr.NewMapper(grpcadapter.ErrorRules()...)
	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(),
			authn.UnaryServerInterceptor(cfg.Auth),
			mapper.UnaryServerInterceptor(logger),
		),
	)
	orderServer, err := grpcadapter.NewOrderServer(grpcadapter.OrderServerDeps{Place: place /* ... */})
	if err != nil {
		return err
	}
	ordersv1.RegisterOrderServiceServer(server, orderServer)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)

	relay, err := outbox.NewRelay(outbox.RelayDeps{DB: db, Publisher: publisher, Logger: logger, Now: time.Now, NewToken: rand.Text}, cfg.Relay)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.GRPCAddr, err)
	}

	// Run everything; the first failure or a signal cancels the rest.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return server.Serve(listener) }) // returns nil after GracefulStop/Stop
	g.Go(func() error { return relay.Run(gctx) })
	g.Go(func() error {
		<-gctx.Done()
		stopSignals() // a second signal now exits immediately
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Total)
		defer cancel()
		return errors.Join(
			stopGRPC(shutdownCtx, server),   // bounded GracefulStop → Stop
			publisher.Close(),
			db.Close(),
			tp.Shutdown(shutdownCtx),        // last, so the failures above are exported
		)
	})
	return g.Wait()
}
```

Order: config → telemetry → infrastructure → adapters → application → transports and workers → run → shut down in reverse. Every constructor error aborts startup with the component name.
