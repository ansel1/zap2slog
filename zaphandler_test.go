package zap2slog

import (
	"context"
	"runtime"
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestZapHandler_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		level    slog.Level
		coreLvl  zapcore.Level
		expected bool
	}{
		{
			name:     "debug enabled",
			level:    slog.LevelDebug,
			coreLvl:  zapcore.DebugLevel,
			expected: true,
		},
		{
			name:     "info disabled at warn level",
			level:    slog.LevelInfo,
			coreLvl:  zapcore.WarnLevel,
			expected: false,
		},
		{
			name:     "warn enabled at warn level",
			level:    slog.LevelWarn,
			coreLvl:  zapcore.WarnLevel,
			expected: true,
		},
		{
			name:     "error always enabled",
			level:    slog.LevelError,
			coreLvl:  zapcore.WarnLevel,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core := &mockCore{enabledLevel: tt.coreLvl}
			h := NewZapHandler(core, nil)
			got := h.Enabled(context.Background(), tt.level)
			assert.Equal(t, tt.expected, got, "ZapHandler.Enabled() = %v, want %v", got, tt.expected)
		})
	}
}

type mockCore struct {
	enabledLevel zapcore.Level
	zapcore.Core
}

func (m *mockCore) Enabled(level zapcore.Level) bool {
	return level >= m.enabledLevel
}

func TestZapHandler_Handle(t *testing.T) {
	pc, file, line, ok := runtime.Caller(0)
	require.True(t, ok)

	tests := []struct {
		name       string
		record     slog.Record
		opts       *ZapHandlerOptions
		coreLvl    zapcore.Level
		wantFields []zapcore.Field
		wantEntry  zapcore.Entry
		wantEmpty  bool
	}{
		{
			name: "basic message",
			record: slog.Record{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
		},
		{
			name: "with attributes",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelWarn,
					Message: "warning message",
				}
				r.AddAttrs(
					slog.String("user", "alice"),
					slog.Int("status", 404),
				)
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.WarnLevel,
				Message: "warning message",
			},
			wantFields: []zapcore.Field{
				zap.String("user", "alice"),
				zap.Int("status", 404),
			},
		},
		{
			name: "with logger name",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				r.AddAttrs(slog.String("logger", "mylogger"))
				return r
			}(),
			opts: &ZapHandlerOptions{
				LoggerNameKey: "logger",
			},
			wantEntry: zapcore.Entry{
				Time:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:      zapcore.InfoLevel,
				Message:    "test message",
				LoggerName: "mylogger",
			},
			wantFields: []zapcore.Field{},
		},
		{
			name: "all value kinds",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				r.AddAttrs(
					slog.String("string", "value"),
					slog.Int("int", 42),
					slog.Int64("int64", 42),
					slog.Float64("float64", 3.14),
					slog.Bool("bool", true),
					slog.Time("time", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
					slog.Duration("duration", 5*time.Second),
					slog.Any("any", []string{"a", "b", "c"}),
					slog.Group("group",
						slog.String("nested", "value"),
						slog.Int("count", 1),
					),
					slog.Uint64("uint64", 42),
				)
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
			wantFields: []zapcore.Field{
				zap.String("string", "value"),
				zap.Int("int", 42),
				zap.Int64("int64", 42),
				zap.Float64("float64", 3.14),
				zap.Bool("bool", true),
				zap.Time("time", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
				zap.Duration("duration", 5*time.Second),
				zap.Any("any", []string{"a", "b", "c"}),
				zap.Dict("group", zap.String("nested", "value"), zap.Int("count", 1)),
				zap.Uint64("uint64", 42),
			},
		},
		{
			name: "disabled level",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelDebug,
					Message: "debug message",
				}
				return r
			}(),
			wantEmpty: true,
		},
		{
			name: "with source",
			opts: &ZapHandlerOptions{
				AddSource: true,
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
					PC:      pc,
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Caller:  zapcore.EntryCaller{Defined: true, PC: pc, File: file, Line: line},
			},
		},
		{
			name: "without source",
			opts: &ZapHandlerOptions{
				AddSource: false,
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
					PC:      pc,
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
		},
		{
			name: "AddSource=true but no valid PC",
			opts: &ZapHandlerOptions{
				AddSource: true,
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
		},
		{
			name:    "debug level",
			coreLvl: zapcore.DebugLevel,
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelDebug,
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.DebugLevel,
				Message: "test message",
			},
		},
		{
			name: "info level",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
		},
		{
			name: "warn level",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelWarn,
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.WarnLevel,
				Message: "test message",
			},
		},
		{
			name: "error level",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelError,
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.ErrorLevel,
				Message: "test message",
			},
		},
		{
			name:    "below debug level",
			coreLvl: zapcore.DebugLevel,
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.Level(-8), // Lower than slog.LevelDebug (-4)
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.DebugLevel,
				Message: "test message",
			},
		},
		{
			name: "between debug and info level",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.Level(-2), // Between slog.LevelDebug (-4) and slog.LevelInfo (0)
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
		},
		{
			name: "between info and warn level",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.Level(2), // Between slog.LevelInfo (0) and slog.LevelWarn (4)
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.WarnLevel,
				Message: "test message",
			},
		},
		{
			name: "between warn and error level",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.Level(6), // Between slog.LevelWarn (4) and slog.LevelError (8)
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.ErrorLevel,
				Message: "test message",
			},
		},
		{
			name: "above error level",
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.Level(10), // Above slog.LevelError (8)
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.ErrorLevel,
				Message: "test message",
			},
		},
		{
			name: "with LogValuer attributes",
			record: func() slog.Record {

				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				r.AddAttrs(
					slog.Any("direct_valuer", logValuerFunc(func() slog.Value {
						return slog.StringValue("resolved_foo")
					})),
					slog.Group("nested",
						slog.Any("nested_valuer", logValuerFunc(func() slog.Value {
							return slog.StringValue("resolved_bar")
						})),
					),
				)
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
			wantFields: []zapcore.Field{
				zap.String("direct_valuer", "resolved_foo"),
				zap.Any("nested", []zapcore.Field{
					zap.String("nested_valuer", "resolved_bar"),
				}),
			},
		},
		{
			name: "LogValuer with ReplaceAttr",
			opts: &ZapHandlerOptions{
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					// Prefix string values with "prefix_" only if they start with "resolved_"
					if a.Value.Kind() == slog.KindString {
						str := a.Value.String()
						if len(str) >= 9 && str[:9] == "resolved_" {
							return slog.String(a.Key, "prefix_"+str)
						}
					}
					return a
				},
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				r.AddAttrs(
					slog.Any("valuer1", logValuerFunc(func() slog.Value {
						return slog.StringValue("resolved_foo")
					})),
					slog.Any("valuer2", logValuerFunc(func() slog.Value {
						return slog.StringValue("unresolved_bar")
					})),
					slog.Group("nested",
						slog.Any("valuer3", logValuerFunc(func() slog.Value {
							return slog.StringValue("resolved_baz")
						})),
					),
				)
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
			wantFields: []zapcore.Field{
				zap.String("valuer1", "prefix_resolved_foo"),
				zap.String("valuer2", "unresolved_bar"),
				zap.Any("nested", []zapcore.Field{
					zap.String("valuer3", "prefix_resolved_baz"),
				}),
			},
		},
		{
			name: "elided attribute from ReplaceAttr",
			opts: &ZapHandlerOptions{
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					// Elide any attribute with key "secret"
					if a.Key == "secret" {
						return slog.Attr{}
					}
					return a
				},
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				r.AddAttrs(
					slog.String("secret", "sensitive data"),
					slog.String("public", "hello"),
					slog.Group("nested",
						slog.String("secret", "hidden"),
						slog.String("visible", "shown"),
					),
				)
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
			wantFields: []zapcore.Field{
				zap.String("public", "hello"),
				zap.Any("nested", []zapcore.Field{
					zap.String("visible", "shown"),
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCore := &mockCoreRecorder{
				mockCore: &mockCore{
					enabledLevel: tt.coreLvl,
				},
			}
			h := NewZapHandler(mockCore, tt.opts)
			err := h.Handle(context.Background(), tt.record)
			require.NoError(t, err)

			if tt.wantEmpty {
				assert.Nil(t, mockCore.lastEntry)
				assert.Nil(t, mockCore.lastFields)
				return
			}

			require.NotNil(t, mockCore.lastEntry)

			got := *mockCore.lastEntry
			want := tt.wantEntry

			assert.Equal(t, want, got)

			gotFields := mockCore.lastFields
			wantFields := tt.wantFields
			assert.Equal(t, wantFields, gotFields)
		})
	}
}

type logValuerFunc func() slog.Value

func (f logValuerFunc) LogValue() slog.Value {
	return f()
}

type mockCoreRecorder struct {
	*mockCore
	lastEntry  *zapcore.Entry
	lastFields []zapcore.Field
}

func (m *mockCoreRecorder) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if m.Enabled(ent.Level) {
		return ce.AddCore(ent, m)
	}
	return ce
}

func (m *mockCoreRecorder) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	m.lastEntry = &ent
	m.lastFields = fields
	return nil
}

func TestZapHandler_WithAttrsAndGroups(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*ZapHandler) slog.Handler
		record     slog.Record
		wantFields []zapcore.Field
		wantEntry  zapcore.Entry
		opts       *ZapHandlerOptions
	}{
		{
			name: "with empty attrs",
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithAttrs(nil)
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
		},
		{
			name: "with attrs only",
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithAttrs([]slog.Attr{
					slog.String("user", "alice"),
					slog.Int("id", 123),
				})
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "test message",
				}
				r.AddAttrs(slog.String("request_id", "req-123"))
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
			wantFields: []zapcore.Field{
				zap.String("user", "alice"),
				zap.Int("id", 123),
				zap.String("request_id", "req-123"),
			},
		},
		{
			name: "with group and attrs",
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithGroup("server").WithAttrs([]slog.Attr{
					slog.String("host", "localhost"),
					slog.Int("port", 8080),
				})
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "server started",
				}
				r.AddAttrs(
					slog.Int("pid", 1234),
					slog.String("version", "1.0.0"),
				)
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "server started",
			},
			wantFields: []zapcore.Field{
				zap.Any("server", []zapcore.Field{
					zap.String("host", "localhost"),
					zap.Int("port", 8080),
					zap.Int("pid", 1234),
					zap.String("version", "1.0.0"),
				}),
			},
		},
		{
			name: "nested groups with attrs",
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithAttrs([]slog.Attr{slog.String("env", "prod")}).
					WithGroup("server").
					WithAttrs([]slog.Attr{slog.String("host", "localhost")}).
					WithGroup("metrics").
					WithAttrs([]slog.Attr{slog.Int("requests", 100)})
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "status report",
				}
				r.AddAttrs(
					slog.Int("memory_mb", 1024),
					slog.Float64("cpu_usage", 0.75),
				)
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "status report",
			},
			wantFields: []zapcore.Field{
				zap.String("env", "prod"),
				zap.Any("server", []zapcore.Field{
					zap.String("host", "localhost"),
					zap.Any("metrics", []zapcore.Field{
						zap.Int("requests", 100),
						zap.Int("memory_mb", 1024),
						zap.Float64("cpu_usage", 0.75),
					}),
				}),
			},
		},
		{
			name: "multiple groups with record attrs in each level",
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithGroup("app").
					WithAttrs([]slog.Attr{slog.String("name", "myapp")}).
					WithGroup("request")
			},
			record: func() slog.Record {
				r := slog.Record{
					Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Level:   slog.LevelInfo,
					Message: "request processed",
				}
				r.AddAttrs(
					slog.String("method", "POST"),
					slog.Int("status", 200),
					slog.Duration("latency", 50*time.Millisecond),
				)
				return r
			}(),
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "request processed",
			},
			wantFields: []zapcore.Field{
				zap.Any("app", []zapcore.Field{
					zap.String("name", "myapp"),
					zap.Any("request", []zapcore.Field{
						zap.String("method", "POST"),
						zap.Int("status", 200),
						zap.Duration("latency", 50*time.Millisecond),
					}),
				}),
			},
		},
		{
			name: "WithAttrs and ReplaceAttr",
			opts: &ZapHandlerOptions{
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					// Prefix all string values with "test_"
					if a.Value.Kind() == slog.KindString {
						return slog.String(a.Key, "test_"+a.Value.String())
					}
					return a
				},
			},
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithAttrs([]slog.Attr{
					slog.String("env", "prod"),
					slog.Int("port", 8080),
				}).WithAttrs([]slog.Attr{
					slog.String("service", "api"),
				})
			},
			record: slog.Record{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   slog.LevelInfo,
				Message: "config loaded",
			},
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "config loaded",
			},
			wantFields: []zapcore.Field{
				zap.String("env", "test_prod"),
				zap.Int("port", 8080),
				zap.String("service", "test_api"),
			},
		},
		{
			name: "WithAttrs with logger name",
			opts: &ZapHandlerOptions{
				LoggerNameKey: "logger",
			},
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithAttrs([]slog.Attr{
					slog.String("logger", "mylogger"),
					slog.String("env", "prod"),
				})
			},
			record: slog.Record{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantEntry: zapcore.Entry{
				Time:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:      zapcore.InfoLevel,
				Message:    "test message",
				LoggerName: "mylogger",
			},
			wantFields: []zapcore.Field{
				zap.String("env", "prod"),
			},
		},
		{
			name: "WithAttrs with only logger name",
			opts: &ZapHandlerOptions{
				LoggerNameKey: "logger",
			},
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithAttrs([]slog.Attr{
					slog.String("logger", "mylogger"),
				})
			},
			record: slog.Record{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantEntry: zapcore.Entry{
				Time:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:      zapcore.InfoLevel,
				Message:    "test message",
				LoggerName: "mylogger",
			},
		},
		{
			name: "WithAttrs with empty group",
			setup: func(h *ZapHandler) slog.Handler {
				return h.WithAttrs([]slog.Attr{
					slog.Group("empty_group"),
				})
			},
			record: slog.Record{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantEntry: zapcore.Entry{
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCore := &mockCoreRecorder{
				mockCore: &mockCore{
					enabledLevel: zapcore.InfoLevel,
				},
			}
			baseHandler := NewZapHandler(mockCore, tt.opts)
			h := tt.setup(baseHandler)

			err := h.Handle(context.Background(), tt.record)
			require.NoError(t, err)
			require.NotNil(t, mockCore.lastEntry)

			got := *mockCore.lastEntry
			assert.Equal(t, tt.wantEntry, got)

			gotFields := mockCore.lastFields
			assert.Equal(t, tt.wantFields, gotFields)
		})
	}
}
