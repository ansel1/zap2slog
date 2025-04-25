package zap2slog

import (
	"context"
	"log/slog"
	"runtime"
	"slices"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapHandlerOptions struct {
	// AddSource adds a source field to the zap log entry.
	AddSource bool
	// ReplaceAttr allows for customizing the attributes of the slog.Record before they are written to the zap log entry.
	// For more information. see slog.HandlerOptions.ReplaceAttr.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
	// LoggerNameKey will search the slog.Record for an attribute with this key.  If found, the zap
	// entry's logger name will be set to the value of that attribute, and the attribute will be elided
	// from the zap entry's fields.
	LoggerNameKey string
}

type ZapHandler struct {
	core       zapcore.Core
	groups     []string
	groupsIdxs []int
	options    ZapHandlerOptions
	loggerName string
	// first dimension maps to open groups
	// len(attrs) must always be len(groups) + 1
	fields []zap.Field
}

func NewZapHandler(core zapcore.Core, opts *ZapHandlerOptions) *ZapHandler {
	if opts == nil {
		opts = &ZapHandlerOptions{}
	}
	return &ZapHandler{
		core:    core,
		options: *opts,
	}
}

func (h *ZapHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.core.Enabled(slogToZapLvl(level))
}

func (h *ZapHandler) Handle(ctx context.Context, record slog.Record) error {

	fields, loggerName := h.toFields(record)

	// apply groups
	for i := len(h.groups) - 1; i >= 0; i-- {
		group := h.groups[i]
		idx := h.groupsIdxs[i]
		subfields := slices.Clone(fields[idx:])
		if len(subfields) > 0 {
			fields = append(fields[:idx], zap.Any(group, subfields))
		}
	}

	entry := h.core.Check(zapcore.Entry{
		Level:      slogToZapLvl(record.Level),
		Time:       record.Time,
		LoggerName: loggerName,
		Message:    record.Message,
	}, nil)

	if entry == nil {
		return nil
	}

	if h.options.AddSource && record.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{record.PC})
		f, _ := fs.Next()
		entry.Caller = zapcore.NewEntryCaller(record.PC, f.File, f.Line, true)
	}

	entry.Write(fields...)

	return nil
}

func (h *ZapHandler) toFields(record slog.Record) ([]zapcore.Field, string) {
	cap := len(h.fields) + record.NumAttrs()
	if cap <= 0 {
		return nil, h.loggerName
	}

	fields := make([]zapcore.Field, len(h.fields), cap)
	copy(fields, h.fields)

	loggerName := h.loggerName

	groupless := len(h.groups) == 0

	record.Attrs(func(a slog.Attr) bool {
		if f, ok := h.attrToField(h.groups, a); ok {
			if groupless && f.Key == h.options.LoggerNameKey && f.Type == zapcore.StringType {
				loggerName = f.String
				// since we're capturing this field as the loggername, elide the field
				return true
			}
			fields = append(fields, f)
		}
		return true
	})

	return fields, loggerName
}

func (h *ZapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields, loggerName := h.attrsToFields(h.groups, attrs)
	if len(fields) == 0 && loggerName == h.loggerName {
		// all attrs ended up being elided and logger name didn't change
		return h
	}
	return &ZapHandler{
		core:       h.core,
		loggerName: loggerName,
		groups:     slices.Clone(h.groups),
		groupsIdxs: slices.Clone(h.groupsIdxs),
		options:    h.options,
		fields:     append(slices.Clone(h.fields), fields...),
	}
}

func (h *ZapHandler) WithGroup(name string) slog.Handler {
	return &ZapHandler{
		core:       h.core,
		loggerName: h.loggerName,
		groups:     append(slices.Clone(h.groups), name),
		groupsIdxs: append(slices.Clone(h.groupsIdxs), len(h.fields)),
		options:    h.options,
		fields:     slices.Clone(h.fields),
	}
}

func slogToZapLvl(zl slog.Level) zapcore.Level {
	switch {
	case zl <= slog.LevelDebug:
		return zapcore.DebugLevel
	case zl <= slog.LevelInfo:
		return zapcore.InfoLevel
	case zl <= slog.LevelWarn:
		return zapcore.WarnLevel
	default:
		return zapcore.ErrorLevel
	}
}

func (h *ZapHandler) resolveAttr(groups []string, a slog.Attr) slog.Attr {

	a.Value = a.Value.Resolve()
	if a.Value.Kind() != slog.KindGroup && h.options.ReplaceAttr != nil {
		a = h.options.ReplaceAttr(groups, a)
		a.Value = a.Value.Resolve()
	}

	return a
}

func (h *ZapHandler) attrsToFields(groups []string, attrs []slog.Attr) ([]zapcore.Field, string) {
	loggerName := h.loggerName

	if len(attrs) == 0 {
		return nil, loggerName
	}

	groupless := len(h.groups) == 0

	fields := make([]zapcore.Field, 0, len(attrs))
	for _, attr := range attrs {
		if field, ok := h.attrToField(groups, attr); ok {
			if groupless && field.Key == h.options.LoggerNameKey && field.Type == zapcore.StringType {
				loggerName = field.String
				// since we're capturing this field as the loggername, elide the field
				continue
			}
			fields = append(fields, field)
		}
	}
	return fields, loggerName
}

func (h *ZapHandler) attrToField(groups []string, attr slog.Attr) (field zapcore.Field, ok bool) {
	// resolve and apply ReplaceAttr
	attr = h.resolveAttr(groups, attr)

	// elide empty attrs
	if attr.Equal(slog.Attr{}) {
		return field, false
	}

	switch attr.Value.Kind() {
	case slog.KindString:
		return zap.String(attr.Key, attr.Value.String()), true
	case slog.KindInt64:
		return zap.Int64(attr.Key, attr.Value.Int64()), true
	case slog.KindUint64:
		return zap.Uint64(attr.Key, attr.Value.Uint64()), true
	case slog.KindFloat64:
		return zap.Float64(attr.Key, attr.Value.Float64()), true
	case slog.KindBool:
		return zap.Bool(attr.Key, attr.Value.Bool()), true
	case slog.KindTime:
		return zap.Time(attr.Key, attr.Value.Time()), true
	case slog.KindDuration:
		return zap.Duration(attr.Key, attr.Value.Duration()), true
	case slog.KindGroup:
		fields, _ := h.attrsToFields(append(groups, attr.Key), attr.Value.Group())
		if len(fields) == 0 {
			return field, false
		}
		return zap.Any(attr.Key, fields), true
	default:
		return zap.Any(attr.Key, attr.Value.Any()), true
	}

}
