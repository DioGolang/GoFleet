package logger

import (
	"context"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

type zapLogger struct {
	log *zap.Logger
}

func NewLogger(serviceName string, isProd bool) Logger {
	var config zapcore.EncoderConfig
	var level zapcore.Level

	if isProd {
		config = zap.NewProductionEncoderConfig()
		level = zapcore.InfoLevel
	} else {
		config = zap.NewProductionEncoderConfig()
		level = zapcore.DebugLevel
	}
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(config),
		zapcore.AddSync(os.Stdout),
		level,
	)
	if isProd {
		core = zapcore.NewSamplerWithOptions(core, 1, 100, 0)
	}
	l := zap.New(core).With(zap.String("service", serviceName))
	return &zapLogger{log: l}
}

func (z *zapLogger) Info(ctx context.Context, msg string, fields ...Field) {
	z.log.Info(msg, z.enrich(ctx, fields)...)
}

func (z *zapLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	z.log.Debug(msg, z.enrich(ctx, fields)...)
}

func (z *zapLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	z.log.Warn(msg, z.enrich(ctx, fields)...)
}

func (z *zapLogger) Error(ctx context.Context, msg string, fields ...Field) {
	z.log.Error(msg, z.enrich(ctx, fields)...)
}

func (z *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{log: z.log.With(z.convertFields(fields)...)}
}

// enrichment com OpenTelemetry
func (z *zapLogger) enrich(ctx context.Context, fields []Field) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields)+2)
	zapFields = append(zapFields, z.convertFields(fields)...)

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		zapFields = append(zapFields,
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return zapFields
}

func (z *zapLogger) convertFields(fields []Field) []zap.Field {
	out := make([]zap.Field, len(fields))
	for i, f := range fields {
		switch f.Kind {
		case KindString:
			if val, ok := f.Value.(string); ok {
				out[i] = zap.String(f.Key, val)
			} else {
				out[i] = zap.Any(f.Key, f.Value)
			}
		case KindInt:
			if val, ok := f.Value.(int); ok {
				out[i] = zap.Int(f.Key, val)
			} else {
				out[i] = zap.Any(f.Key, f.Value)
			}
		case KindError:
			if val, ok := f.Value.(error); ok {
				out[i] = zap.Error(val)
			} else {
				out[i] = zap.Any(f.Key, f.Value)
			}
		default:
			out[i] = zap.Any(f.Key, f.Value)
		}
	}
	return out
}
