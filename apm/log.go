package apm

import (
	"context"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

/**
  *  @author tryao
  *  @date 2023/01/03 11:34
**/

func doLog(ctx context.Context, f func(string, ...any), format string, a ...any) {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() && spanCtx.HasSpanID() {
		prefix := fmt.Sprintf("\t%s\t%s", spanCtx.TraceID().String(),
			spanCtx.SpanID().String())
		f(prefix+format, a...)
	} else {
		f(format, a...)
	}
}

func doJsonLog(ctx context.Context, f func(msg string, fields ...zap.Field), msg string, fields ...zap.Field) {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() && spanCtx.HasSpanID() {
		fields = append(fields,
			zap.String("traceId", spanCtx.TraceID().String()),
			zap.String("spanId", spanCtx.SpanID().String()))
	}
	f(msg, fields...)
}

func Debug(ctx context.Context, format string, a ...any) {
	doLog(ctx, log.Debug, format, a...)
}

func Info(ctx context.Context, format string, a ...any) {
	doLog(ctx, log.Info, format, a...)
}

func Warn(ctx context.Context, format string, a ...any) {
	doLog(ctx, log.Warn, format, a...)
}

func Error(ctx context.Context, format string, a ...any) {
	doLog(ctx, log.Error, format, a...)
}

func Fatal(ctx context.Context, format string, a ...any) {
	doLog(ctx, log.Fatal, format, a...)
}

func JsonDebug(ctx context.Context, msg string, fields ...zap.Field) {
	doJsonLog(ctx, log.JsonDebug, msg, fields...)
}

func JsonInfo(ctx context.Context, msg string, fields ...zap.Field) {
	doJsonLog(ctx, log.JsonInfo, msg, fields...)
}

func JsonWarn(ctx context.Context, msg string, fields ...zap.Field) {
	doJsonLog(ctx, log.JsonWarn, msg, fields...)
}

func JsonError(ctx context.Context, msg string, fields ...zap.Field) {
	doJsonLog(ctx, log.JsonError, msg, fields...)
}
