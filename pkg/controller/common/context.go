package common

import (
	"context"

	"github.com/go-logr/logr"
)

type ContextValueKey string

var (
	logContextKey ContextValueKey = "github.com/maistra/istio-operator/pkg/controller/common/logr.Logger"
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

// LogFromContext returns the Logger bound to the context or panics if none exists
func LogFromContext(ctx context.Context) logr.Logger {
	logger, ok := ctx.Value(logContextKey).(logr.Logger)
	if !ok {
		panic("Could not get Logger from context")
	}
	return logger
}
