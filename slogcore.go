package zap2slog

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"go.uber.org/zap/zapcore"
)

// SlogCore implements zapcore.Core
var _ zapcore.Core = (*SlogCore)(nil)

type SlogCoreOptions struct {
	// LoggerNameKey adds an attribute to slog.Records containing the zap logger name.
	// If LoggerNameKey is empty, or the zap logger name is empty, then no attribute is added.
	LoggerNameKey string
}

type SlogCore struct {
	h      slog.Handler
	opts   SlogCoreOptions
	fields []zapcore.Field
}

func NewSlogCore(h slog.Handler, opts *SlogCoreOptions) *SlogCore {
	if opts == nil {
		opts = &SlogCoreOptions{}
	}
	return &SlogCore{
		h:    h,
		opts: *opts,
	}
}

func (c *SlogCore) Enabled(l zapcore.Level) bool {
	return c.h.Enabled(context.Background(), zapToSlogLvl(l))
}

func (c *SlogCore) With(fields []zapcore.Field) zapcore.Core {
	if len(fields) == 0 {
		return c
	}
	// can't translate to calls to slog.Handler.WithAttrs or WithGroup
	// That's because in Write, we try and translate the logger name
	// into a slog attribute, but it should not be part of any open
	// groups...if I call WithGroup() here, I'll end up with a
	// slog.Handler with open groups in the Write() call, and I can't
	// add any non-group-scoped attributes at that point.
	return &SlogCore{
		h:      c.h,
		opts:   c.opts,
		fields: slices.Clip(append(c.fields, fields...)),
	}
}

func (c *SlogCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}
	return nil
}

func (c *SlogCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	var pc uintptr
	if e.Caller.Defined {
		pc = e.Caller.PC
	}

	rec := slog.NewRecord(e.Time, zapToSlogLvl(e.Level), e.Message, pc)

	if c.opts.LoggerNameKey != "" && e.LoggerName != "" {
		rec.AddAttrs(slog.String(c.opts.LoggerNameKey, e.LoggerName))
	}

	fields = append(c.fields, fields...)

	var enc slogObjEnc
	for _, f := range fields {
		f.AddTo(&enc)
	}

	rec.AddAttrs(enc.finalAttrs()...)

	return c.h.Handle(context.Background(), rec)
}

func (c *SlogCore) Sync() error {
	return nil
}

func zapToSlogLvl(zl zapcore.Level) slog.Level {
	switch zl {
	case zapcore.DebugLevel:
		return slog.LevelDebug
	case zapcore.InfoLevel:
		return slog.LevelInfo
	case zapcore.WarnLevel:
		return slog.LevelWarn
	case zapcore.ErrorLevel:
		return slog.LevelError
	}
	if zl < zapcore.DebugLevel {
		return slog.LevelDebug
	} else {
		return slog.LevelError
	}
}

const nAttrsInline = 5

type slogObjEnc struct {
	inlineAttrs [nAttrsInline]slog.Attr
	attrs       []slog.Attr
	groups      []string
	groupIdxs   []int
}

func (s *slogObjEnc) append(attr slog.Attr) {
	// avoid allocation if possible
	if s.attrs == nil {
		s.attrs = s.inlineAttrs[:0]
	}
	s.attrs = append(s.attrs, attr)
}

func (s *slogObjEnc) finalAttrs() []slog.Attr {
	// apply groups
	for i := len(s.groups) - 1; i >= 0; i-- {
		group := s.groups[i]
		idx := s.groupIdxs[i]
		groupMembers := slices.Clone(s.attrs[idx:])
		if len(groupMembers) > 0 {
			s.attrs = append(s.attrs[:idx], slog.Attr{Key: group, Value: slog.GroupValue(groupMembers...)})
		}
	}

	return s.attrs
}

func (s *slogObjEnc) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	senc := sliceArrayEncoder{}
	err := marshaler.MarshalLogArray(&senc)
	if err != nil {
		return err
	}
	if len(senc.elems) > 0 {
		s.append(slog.Any(key, senc.elems))
	}
	return nil
}

func (s *slogObjEnc) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	var s2 slogObjEnc
	err := marshaler.MarshalLogObject(&s2)
	if err != nil {
		return err
	}
	attrs := s2.finalAttrs()
	if len(attrs) > 0 {
		s.append(slog.Any(key, attrs))
	}
	return nil
}

func (s *slogObjEnc) AddBinary(key string, value []byte) {
	s.append(slog.Any(key, value))
}

func (s *slogObjEnc) AddByteString(key string, value []byte) {
	s.append(slog.String(key, string(value)))
}

func (s *slogObjEnc) AddBool(key string, value bool) {
	s.append(slog.Bool(key, value))
}

func (s *slogObjEnc) AddComplex128(key string, value complex128) {
	s.append(slog.Any(key, value))
}

func (s *slogObjEnc) AddComplex64(key string, value complex64) {
	s.append(slog.Any(key, value))
}

func (s *slogObjEnc) AddDuration(key string, value time.Duration) {
	s.append(slog.Duration(key, value))
}

func (s *slogObjEnc) AddFloat64(key string, value float64) {
	s.append(slog.Float64(key, value))
}

func (s *slogObjEnc) AddFloat32(key string, value float32) {
	s.append(slog.Float64(key, float64(value)))
}

// AddInt can't be tested because it's never called.  zap defined this as
// part of the ObjectEncoder interface, but it's never
// actually used in zap (AddInt64 is used instead).
func (s *slogObjEnc) AddInt(key string, value int) {
	s.append(slog.Int(key, value))
}

func (s *slogObjEnc) AddInt64(key string, value int64) {
	s.append(slog.Int64(key, value))
}

func (s *slogObjEnc) AddInt32(key string, value int32) {
	s.append(slog.Int(key, int(value)))
}

func (s *slogObjEnc) AddInt16(key string, value int16) {
	s.append(slog.Int(key, int(value)))
}

func (s *slogObjEnc) AddInt8(key string, value int8) {
	s.append(slog.Int(key, int(value)))
}

func (s *slogObjEnc) AddString(key string, value string) {
	s.append(slog.String(key, value))
}

func (s *slogObjEnc) AddTime(key string, value time.Time) {
	s.append(slog.Time(key, value))
}

// AddUint can't be tested because it's never called.  zap defined this as
// part of the ObjectEncoder interface, but it's never
// actually used in zap (AddUint64 is used instead).
func (s *slogObjEnc) AddUint(key string, value uint) {
	s.append(slog.Uint64(key, uint64(value)))
}

func (s *slogObjEnc) AddUint64(key string, value uint64) {
	s.append(slog.Uint64(key, value))
}

func (s *slogObjEnc) AddUint32(key string, value uint32) {
	s.append(slog.Uint64(key, uint64(value)))
}

func (s *slogObjEnc) AddUint16(key string, value uint16) {
	s.append(slog.Uint64(key, uint64(value)))
}

func (s *slogObjEnc) AddUint8(key string, value uint8) {
	s.append(slog.Uint64(key, uint64(value)))
}

func (s *slogObjEnc) AddUintptr(key string, value uintptr) {
	s.append(slog.Any(key, value))
}

func (s *slogObjEnc) AddReflected(key string, value interface{}) error {
	s.append(slog.Any(key, value))
	return nil
}

func (s *slogObjEnc) OpenNamespace(key string) {
	// open a new group
	s.groups = append(s.groups, key)
	s.groupIdxs = append(s.groupIdxs, len(s.attrs))
}

// sliceArrayEncoder implements zapcore.ArrayMarshaler, and marshals the value
// into a slice of any.
type sliceArrayEncoder struct {
	elems []interface{}
}

func (s *sliceArrayEncoder) AppendArray(v zapcore.ArrayMarshaler) error {
	enc := &sliceArrayEncoder{}
	err := v.MarshalLogArray(enc)
	s.elems = append(s.elems, enc.elems)
	return err
}

func (s *sliceArrayEncoder) AppendObject(v zapcore.ObjectMarshaler) error {
	m := zapcore.NewMapObjectEncoder()
	err := v.MarshalLogObject(m)
	s.elems = append(s.elems, m.Fields)
	return err
}

func (s *sliceArrayEncoder) AppendReflected(v interface{}) error {
	s.elems = append(s.elems, v)
	return nil
}

func (s *sliceArrayEncoder) AppendBool(v bool)              { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendByteString(v []byte)      { s.elems = append(s.elems, string(v)) }
func (s *sliceArrayEncoder) AppendComplex128(v complex128)  { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendComplex64(v complex64)    { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendDuration(v time.Duration) { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendFloat64(v float64)        { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendFloat32(v float32)        { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt(v int)                { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt64(v int64)            { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt32(v int32)            { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt16(v int16)            { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt8(v int8)              { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendString(v string)          { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendTime(v time.Time)         { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint(v uint)              { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint64(v uint64)          { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint32(v uint32)          { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint16(v uint16)          { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint8(v uint8)            { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUintptr(v uintptr)        { s.elems = append(s.elems, v) }
