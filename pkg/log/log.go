package log

import (
	"context"
	"go.opentelemetry.io/otel/attribute"

	logrus "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

// Fatalf Logs first and then calls `logger.Exit(1)`
// logging level is set to Panic.
func Fatalf(msg string, err ...interface{}) {
	logrus.WithFields(logrus.Fields{}).Fatalf(msg, err...)
}

// Fatal Logs first and then calls `logger.Exit(1)`
// logging level is set to Panic.
func Fatal(msg string) {
	logrus.WithFields(logrus.Fields{}).Fatal(msg)
}

// Infof log the General operational entries about what's going on inside the application
func Infof(msg string, val ...interface{}) {
	logrus.WithFields(logrus.Fields{}).Infof(msg, val...)
}

// Info log the General operational entries about what's going on inside the application
func Info(msg string) {
	logrus.WithFields(logrus.Fields{}).Infof(msg)
}

// InfoWithValues log the General operational entries about what's going on inside the application
// It also print the extra key values pairs
func InfoWithValues(msg string, val map[string]interface{}) {
	logrus.WithFields(val).Info(msg)
}

// ErrorWithValues log the Error entries happening inside the code
// It also print the extra key values pairs
func ErrorWithValues(msg string, val map[string]interface{}) {
	logrus.WithFields(val).Error(msg)
}

// Warn log the Non-critical entries that deserve eyes.
func Warn(msg string) {
	logrus.WithFields(logrus.Fields{}).Warn(msg)
}

// Warnf log the Non-critical entries that deserve eyes.
func Warnf(msg string, val ...interface{}) {
	logrus.WithFields(logrus.Fields{}).Warnf(msg, val...)
}

// Errorf used for errors that should definitely be noted.
// Commonly used for hooks to send errors to an error tracking service.
func Errorf(msg string, err ...interface{}) {
	logrus.WithFields(logrus.Fields{}).Errorf(msg, err...)
}

// Error used for errors that should definitely be noted.
// Commonly used for hooks to send errors to an error tracking service
func Error(msg string) {
	logrus.WithFields(logrus.Fields{}).Error(msg)
}

func WithContext(ctx context.Context) ContextEntry {
	return ContextEntry{logrus.WithContext(ctx)}
}

type ContextEntry struct {
	*logrus.Entry
}

// Fatalf Logs first and then calls `logger.Exit(1)`
// logging level is set to Panic.
func (e ContextEntry) Fatalf(msg string, err ...interface{}) {
	e.WithFields(logrus.Fields{}).Fatalf(msg, err...)
}

// Fatal Logs first and then calls `logger.Exit(1)`
// logging level is set to Panic.
func (e ContextEntry) Fatal(msg string) {
	e.WithFields(logrus.Fields{}).Fatal(msg)
}

// Infof log the General operational entries about what's going on inside the application
func (e ContextEntry) Infof(msg string, val ...interface{}) {
	e.WithFields(logrus.Fields{}).Infof(msg, val...)
}

// Info log the General operational entries about what's going on inside the application
func (e ContextEntry) Info(msg string) {
	e.WithFields(logrus.Fields{}).Infof(msg)
}

// InfoWithValues log the General operational entries about what's going on inside the application
// It also print the extra key values pairs
func (e ContextEntry) InfoWithValues(msg string, val map[string]interface{}) {
	e.WithFields(val).Info(msg)
}

// ErrorWithValues log the Error entries happening inside the code
// It also print the extra key values pairs
func (e ContextEntry) ErrorWithValues(msg string, val map[string]interface{}) {
	e.WithFields(val).Error(msg)
}

// Warn log the Non-critical entries that deserve eyes.
func (e ContextEntry) Warn(msg string) {
	e.WithFields(logrus.Fields{}).Warn(msg)
}

// Warnf log the Non-critical entries that deserve eyes.
func (e ContextEntry) Warnf(msg string, val ...interface{}) {
	e.WithFields(logrus.Fields{}).Warnf(msg, val...)
}

// Errorf used for errors that should definitely be noted.
// Commonly used for hooks to send errors to an error tracking service.
func (e ContextEntry) Errorf(msg string, err ...interface{}) {
	e.WithFields(logrus.Fields{}).Errorf(msg, err...)
}

// Error used for errors that should definitely be noted.
// Commonly used for hooks to send errors to an error tracking service
func (e ContextEntry) Error(msg string) {
	e.WithFields(logrus.Fields{}).Error(msg)
}

type SpanLogHook struct{}

func (h *SpanLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *SpanLogHook) Fire(entry *logrus.Entry) error {
	// Context에서 Span을 가져옴
	span := trace.SpanFromContext(entry.Context)
	if span != nil && span.IsRecording() { // 스팬이 기록 중인 경우
		ctx := span.SpanContext()

		// traceID와 spanID 추가
		if ctx.HasTraceID() {
			entry.Data["traceID"] = ctx.TraceID().String()
		}
		if ctx.HasSpanID() {
			entry.Data["spanID"] = ctx.SpanID().String()
		}

		// 로그 메시지를 span event로 추가
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("level", entry.Level.String()),
			attribute.String("message", entry.Message),
		))
	}
	return nil
}
