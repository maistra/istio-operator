package common

import (
	"context"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type ContextValueKey string

var (
	logContextKey ContextValueKey = "github.com/maistra/istio-operator/pkg/controller/common/logr.Logger"

	fallBackLogger = logf.Log.WithName("FALLBACK-LOGGER")
)

// NewContext creates a new context without an associated Logger
func NewContext() context.Context {
	return context.Background()
}

// NewReconcileContext creates a new context
// and associates it with a Logger.
func NewReconcileContext(logger logr.Logger) context.Context {
	return NewContextWithLog(NewContext(), logger)
}

// NewContextWithLog returns a copy of the parent context
// and associates it with a Logger.
func NewContextWithLog(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, logContextKey, logger)
}

// NewContextWithLogValues returns a copy of the parent context
// and adds the given keys and values to its logger
func NewContextWithLogValues(ctx context.Context, logKeysAndValues ...interface{}) context.Context {
	log := LogFromContext(ctx).WithValues(logKeysAndValues)
	return NewContextWithLog(ctx, log)
}

// LogFromContext returns the Logger bound to the context or panics if none exists
func LogFromContext(ctx context.Context) logr.Logger {
	logger, ok := ctx.Value(logContextKey).(logr.Logger)
	if !ok {
		return fallBackLogger
	}
	return logger
}
