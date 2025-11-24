package runtime

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/crush/internal/telemetry"
)

// TelemetryLoggerRegistrar subscribes to telemetry and logs to slog for now.
type TelemetryLoggerRegistrar struct{}

func (TelemetryLoggerRegistrar) Register(ctx context.Context, deps Services) error {
	rec, ok := deps.Telemetry.(*telemetry.Recorder)
	if !ok || rec == nil {
		return nil
	}
	ch := rec.Subscribe(ctx)
	go func() {
		for evt := range ch {
			slog.Debug("telemetry",
				"type", evt.Payload.Type,
				"session", evt.Payload.SessionID,
				"tool", evt.Payload.ToolName,
				"duration", evt.Payload.Duration,
				"cost", evt.Payload.Cost,
				"tokens_in", evt.Payload.TokensIn,
				"tokens_out", evt.Payload.TokensOut,
			)
		}
	}()
	return nil
}
