zap2slog is a library that adapts zap to slog, and slog to zap.

## Usage

### zap to slog

Use `zap2slog.NewSlogCore` to create a zapcore.Core that writes to a slog.Handler.

```go
core := zapcore.NewCore(zap2slog.NewSlogCore(slog.Default().Handler()), nil, zap.NewAtomicLevelAt(zapcore.InfoLevel))
l := zap.New(core)
l.Info("hello, world")
```

Zap loggers have a name, which has no equivalent in slog.  Set `zap2slog.SlogCoreOptions.LoggerNameKey` to add an attribute to
slog.Records with the logger name.

### slog to zap

Use `zap2slog.NewZapHandler` to create a slog.Handler that writes to a zapcore.Core.

```go
    zl, _ := zap.NewProductionConfig().Build()
    h := zap2slog.NewZapHandler(zl.Core(), nil)
    slog.New(h).Info("hello, world")
```
Zap loggers have a name, which has no equivalent in slog.  Set `zap2slog.ZapHandlerOptions.LoggerNameKey` extract one of
the slog.Record's attributes and use it as the zap logger name.

`ZapHandler` also supports AddSource and ReplaceAttr options, which behavior like slog.HandlerOptions.AddSource and slog.HandlerOptions.ReplaceAttr.