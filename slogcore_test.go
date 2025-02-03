package zap2slog

import (
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSlogCore_Enabled(t *testing.T) {
	var lvl slog.LevelVar
	h := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: &lvl})
	core := NewSlogCore(h, &SlogCoreOptions{
		LoggerNameKey: "logger",
	})

	lvl.Set(slog.LevelDebug)

	require.True(t, core.Enabled(zapcore.DebugLevel))

	lvl.Set(slog.LevelWarn)

	require.False(t, core.Enabled(zapcore.DebugLevel))
	require.False(t, core.Enabled(zapcore.InfoLevel))
	require.True(t, core.Enabled(zapcore.WarnLevel))
}

func TestSlogCore_Sync(t *testing.T) {
	h := slog.NewTextHandler(io.Discard, nil)
	core := NewSlogCore(h, nil)

	err := core.Sync()
	require.NoError(t, err)
}

func TestSlogCore_Check(t *testing.T) {
	h := slog.NewTextHandler(io.Discard, nil)
	core := NewSlogCore(h, nil)

	tests := []struct {
		name    string
		entry   zapcore.Entry
		wantNil bool
	}{
		{
			name: "enabled level",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
			wantNil: false,
		},
		{
			name: "disabled level",
			entry: zapcore.Entry{
				Level:   zapcore.DebugLevel,
				Message: "debug message",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ce *zapcore.CheckedEntry
			got := core.Check(tt.entry, ce)

			if tt.wantNil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
			}
		})
	}
}

func TestSlogCore_Write(t *testing.T) {
	pc, file, line, ok := runtime.Caller(0)
	require.True(t, ok)
	wantSource := fmt.Sprintf("%s:%d", file, line)

	tests := []struct {
		name      string
		opts      *SlogCoreOptions
		with      []zapcore.Field
		entry     zapcore.Entry
		fields    []zapcore.Field
		want      string
		addSource bool
	}{
		{
			name: "basic message",
			opts: &SlogCoreOptions{},
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "hello world",
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"hello world\"\n",
		},
		{
			name: "with logger name",
			opts: &SlogCoreOptions{
				LoggerNameKey: "logger",
			},
			entry: zapcore.Entry{
				Level:      zapcore.WarnLevel,
				Time:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message:    "warning message",
				LoggerName: "mylogger",
			},
			want: "time=2024-01-01T12:00:00.000Z level=WARN msg=\"warning message\" logger=mylogger\n",
		},
		{
			name: "lower than debug level",
			entry: zapcore.Entry{
				Level:   zapcore.DebugLevel - 1,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "debug message",
			},
			want: "time=2024-01-01T12:00:00.000Z level=DEBUG msg=\"debug message\"\n",
		},
		{
			name: "debug level",
			entry: zapcore.Entry{
				Level:   zapcore.DebugLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "debug message",
			},
			want: "time=2024-01-01T12:00:00.000Z level=DEBUG msg=\"debug message\"\n",
		},
		{
			name: "info level",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "info message",
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"info message\"\n",
		},
		{
			name: "warn level",
			entry: zapcore.Entry{
				Level:   zapcore.WarnLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "warn message",
			},
			want: "time=2024-01-01T12:00:00.000Z level=WARN msg=\"warn message\"\n",
		},
		{
			name: "error level",
			entry: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "error message",
			},
			want: "time=2024-01-01T12:00:00.000Z level=ERROR msg=\"error message\"\n",
		},
		{
			name: "dpanic level maps to error",
			entry: zapcore.Entry{
				Level:   zapcore.DPanicLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "dpanic message",
			},
			want: "time=2024-01-01T12:00:00.000Z level=ERROR msg=\"dpanic message\"\n",
		},
		{
			name: "panic level maps to error",
			entry: zapcore.Entry{
				Level:   zapcore.PanicLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "panic message",
			},
			want: "time=2024-01-01T12:00:00.000Z level=ERROR msg=\"panic message\"\n",
		},
		{
			name: "fatal level maps to error",
			entry: zapcore.Entry{
				Level:   zapcore.FatalLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "fatal message",
			},
			want: "time=2024-01-01T12:00:00.000Z level=ERROR msg=\"fatal message\"\n",
		},
		{
			name: "with fields",
			entry: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "error occurred",
			},
			fields: []zapcore.Field{
				zap.String("user", "alice"),
				zap.Int("count", 42),
			},
			want: "time=2024-01-01T12:00:00.000Z level=ERROR msg=\"error occurred\" user=alice count=42\n",
		},
		{
			name: "with complex fields",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "complex data",
			},
			fields: []zapcore.Field{
				zap.Duration("latency", 50*time.Millisecond),
				zap.Binary("data", []byte("hello")),
				zap.Time("event_time", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"complex data\" latency=50ms data=\"hello\" event_time=2024-01-01T12:00:00.000Z\n",
		},
		{
			name: "with namespaces",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "nested data",
			},
			fields: []zapcore.Field{
				zap.Namespace("request"),
				zap.String("method", "POST"),
				zap.Int("status", 200),
				zap.Namespace("user"),
				zap.String("id", "123"),
				zap.String("name", "alice"),
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"nested data\" request.method=POST request.status=200 request.user.id=123 request.user.name=alice\n",
		},
		{
			name: "with pre-added fields",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "message with context",
			},
			with: []zapcore.Field{
				zap.String("env", "prod"),
				zap.Int("instance", 1),
			},
			fields: []zapcore.Field{
				zap.String("action", "test"),
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"message with context\" env=prod instance=1 action=test\n",
		},
		{
			name: "with pre-added fields and namespaces",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "message with context",
			},
			with: []zapcore.Field{
				zap.String("env", "prod"),
				zap.Namespace("request"),
				zap.Int("instance", 1),
			},
			fields: []zapcore.Field{
				zap.String("action", "test"),
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"message with context\" env=prod request.instance=1 request.action=test\n",
		},
		{
			name: "testing every zap value type",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "message with context",
			},
			fields: []zapcore.Field{
				zap.String("string", "hello"),
				zap.Binary("binary", []byte("binary data")),
				zap.ByteString("byteString", []byte("byteString data")),
				zap.Int("int", 42),
				zap.Int64("int64", 42),
				zap.Int32("int32", 42),
				zap.Int16("int16", 42),
				zap.Int8("int8", 42),
				zap.Uint("uint", 42),
				zap.Uint64("uint64", 42),
				zap.Uint32("uint32", 42),
				zap.Uint16("uint16", 42),
				zap.Uint8("uint8", 42),
				zap.Uintptr("uintptr", 42),
				zap.Float64("float64", 3.14),
				zap.Float32("float32", float32(3)),
				zap.Bool("bool", true),
				zap.Time("time", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
				zap.Duration("duration", 10*time.Second),
				zap.Reflect("reflect", struct{ Name string }{Name: "reflect"}),

				zap.Strings("strings", []string{"hello", "world"}),                      // this tests ArrayMarshalers
				zap.Dict("dict", zap.String("size", "big"), zap.String("color", "red")), // this tests ObjectMarshalers
				zap.Dict("dict2",
					zap.Objects("objs", []zapcore.ObjectMarshaler{
						dictObject{zap.String("color", "red")},
						dictObject{zap.String("color", "blue"), zap.Bools("bools", []bool{true, false})},
					}),
				),
				zap.Any("nestedarrays", zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
					enc.AppendString("hello")
					return enc.AppendArray(zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
						enc.AppendString("world")
						return nil
					}))
				})), // tests nested Object and ArrayMarshalers
				zap.Inline(dictObject{zap.String("inlinekey", "inlinevalue")}), // tests InlineMarshalers
				zap.Complex128("complex128", complex(1, 2)),
				zap.Complex64("complex64", complex(1, 2)),
				zap.Namespace("namespace"),
				zap.Any("any", "any value"),
			},
			want: strings.Join([]string{
				`time=2024-01-01T12:00:00.000Z level=INFO msg="message with context"`,
				`string=hello`,
				`binary="binary data"`,
				`byteString="byteString data"`,
				`int=42`,
				`int64=42`,
				`int32=42`,
				`int16=42`,
				`int8=42`,
				`uint=42`,
				`uint64=42`,
				`uint32=42`,
				`uint16=42`,
				`uint8=42`,
				`uintptr=42`,
				`float64=3.14`,
				`float32=3`,
				`bool=true`,
				`time=2024-01-01T12:00:00.000Z`,
				`duration=10s`,
				`reflect={Name:reflect}`,
				`strings="[hello world]"`,
				`dict.size=big dict.color=red`,
				`dict2.objs="[map[color:red] map[bools:[true false] color:blue]]"`,
				`nestedarrays="[hello [world]]"`,
				`inlinekey=inlinevalue`,
				`complex128=(1+2i)`,
				`complex64=(1+2i)`,
				`namespace.any="any value"`,
			}, " ") + "\n",
		},
		{
			name: "array marshaler with all types",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "array test",
			},
			fields: []zapcore.Field{
				zap.Array("array", zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
					enc.AppendBool(true)
					enc.AppendByteString([]byte("bytes"))
					enc.AppendComplex128(complex(1, 2))
					enc.AppendComplex64(complex(3, 4))
					enc.AppendFloat64(3.14159)
					enc.AppendFloat32(2.71828)
					enc.AppendInt(42)
					enc.AppendInt64(9223372036854775807)
					enc.AppendInt32(2147483647)
					enc.AppendInt16(32767)
					enc.AppendInt8(127)
					enc.AppendString("string")
					enc.AppendUint(42)
					enc.AppendUint64(18446744073709551615)
					enc.AppendUint32(4294967295)
					enc.AppendUint16(65535)
					enc.AppendUint8(255)
					enc.AppendDuration(time.Hour)
					enc.AppendTime(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
					_ = enc.AppendReflected(struct{ Name string }{Name: "reflect"})
					_ = enc.AppendObject(dictObject{zap.String("dictkey", "dictvalue")})
					_ = enc.AppendArray(zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
						enc.AppendString("hello")
						return enc.AppendArray(zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
							enc.AppendString("world")
							return nil
						}))
					}))
					return nil
				})),
			},
			want: strings.Join([]string{
				`time=2024-01-01T12:00:00.000Z level=INFO msg="array test"`,
				`array="[true bytes (1+2i) (3+4i) 3.14159 2.71828 42 9223372036854775807 2147483647 32767 127 string 42 18446744073709551615 4294967295 65535 255 1h0m0s 2024-01-01 12:00:00 +0000 UTC {Name:reflect} map[dictkey:dictvalue] [hello [world]]]"`,
			}, " ") + "\n",
		},
		{
			name: "empty logger name key",
			opts: &SlogCoreOptions{
				LoggerNameKey: "",
			},
			entry: zapcore.Entry{
				Level:      zapcore.InfoLevel,
				Time:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message:    "test message",
				LoggerName: "mylogger",
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"test message\"\n",
		},
		{
			name: "custom logger name key",
			opts: &SlogCoreOptions{
				LoggerNameKey: "log_source",
			},
			entry: zapcore.Entry{
				Level:      zapcore.InfoLevel,
				Time:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message:    "test message",
				LoggerName: "mylogger",
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"test message\" log_source=mylogger\n",
		},
		{
			name:      "with source info",
			addSource: true,
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "test message",
				Caller:  zapcore.EntryCaller{Defined: true, PC: pc},
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO source=" + wantSource + " msg=\"test message\"\n",
		},
		{
			name:      "without source info",
			addSource: true,
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "test message",
				Caller:  zapcore.EntryCaller{Defined: true, PC: 0},
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO source=:0 msg=\"test message\"\n",
		},
		{
			name:      "with source info but undefined caller",
			addSource: true,
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "test message",
				Caller:  zapcore.EntryCaller{Defined: false},
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO source=:0 msg=\"test message\"\n",
		},
		{
			name: "object marshaler error",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "test message",
			},
			fields: []zapcore.Field{
				zap.Object("obj", zapcore.ObjectMarshalerFunc(func(enc zapcore.ObjectEncoder) error {
					return fmt.Errorf("marshal error")
				})),
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"test message\" objError=\"marshal error\"\n",
		},
		{
			name: "array marshaler error",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Message: "test message",
			},
			fields: []zapcore.Field{
				zap.Array("arr", zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
					return fmt.Errorf("array marshal error")
				})),
			},
			want: "time=2024-01-01T12:00:00.000Z level=INFO msg=\"test message\" arrError=\"array marshal error\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			h := slog.NewTextHandler(&buf, &slog.HandlerOptions{
				AddSource: tt.addSource,
				Level:     slog.LevelDebug,
			})

			core := NewSlogCore(h, tt.opts).With(tt.with)

			ce := core.Check(tt.entry, nil)
			ce.Write(tt.fields...)

			got := buf.String()
			require.Equal(t, tt.want, got)
		})
	}
}

type dictObject []zapcore.Field

func (d dictObject) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for _, f := range d {
		f.AddTo(enc)
	}
	return nil
}

func BenchmarkSlogCore(b *testing.B) {
	h := slog.NewTextHandler(io.Discard, nil)
	core := NewSlogCore(h, nil)
	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Now(),
		Message: "benchmark",
	}

	fields := []zapcore.Field{
		zap.String("method", "POST"),
		zap.Int("status", 200),
		zap.String("id", "123"),
		zap.String("name", "alice"),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ce := core.Check(entry, nil)
		ce.Write(fields...)
	}
}
