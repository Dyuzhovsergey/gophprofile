package logging

import (
	"context"
	"log/slog"
	"strings"
)

const (
	// ComponentHTTP обозначает HTTP-слой приложения.
	ComponentHTTP = "http"

	// ComponentService обозначает service-слой приложения.
	ComponentService = "service"

	// ComponentWorker обозначает worker-слой приложения.
	ComponentWorker = "worker"

	// ComponentOutbox обозначает outbox dispatcher.
	ComponentOutbox = "outbox"

	// ComponentRabbitMQ обозначает RabbitMQ broker layer.
	ComponentRabbitMQ = "rabbitmq"

	// ComponentPostgres обозначает PostgreSQL repository layer.
	ComponentPostgres = "postgres"

	// ComponentS3 обозначает S3/MinIO storage layer.
	ComponentS3 = "s3"

	// ComponentApp обозначает запуск и остановку приложения.
	ComponentApp = "app"
)

// ErrorAttrs возвращает единый набор slog-атрибутов для error-логов.
//
// В атрибуты добавляются:
//   - trace_id/span_id
//   - component;
//   - operation;
//   - error;
//   - user_id/avatar_id.
func ErrorAttrs(
	ctx context.Context,
	component string,
	operation string,
	err error,
	attrs ...slog.Attr,
) []slog.Attr {
	result := make([]slog.Attr, 0, len(attrs)+5)

	result = append(result,
		slog.String("component", strings.TrimSpace(component)),
		slog.String("operation", strings.TrimSpace(operation)),
	)

	if err != nil {
		result = append(result, slog.Any("error", err))
	}

	result = append(result, attrs...)

	return AppendTraceAttrs(ctx, result...)
}
