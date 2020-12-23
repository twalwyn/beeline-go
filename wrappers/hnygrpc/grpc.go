package hnygrpc

import (
	"context"
	"reflect"
	"runtime"

	"github.com/honeycombio/beeline-go/trace"
	"github.com/honeycombio/beeline-go/wrappers/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func getMetadataStringValue(md metadata.MD, key string) string {
	if val, ok := md[key]; ok {
		if len(val) > 0 {
			return val[0]
		}
		return ""
	}
	return ""
}

func startSpanOrTraceFromUnaryGRPC(
	ctx context.Context,
	info *grpc.UnaryServerInfo,
	parserHook config.GRPCTraceParserHook,
) (context.Context, *trace.Span) {
	span := trace.GetSpanFromContext(ctx)
	if span == nil {
		// there is no trace yet. We should maake one! and use the root span.
		var tr *trace.Trace
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			if parserHook == nil {
				beelineHeader := getMetadataStringValue(md, "x-honeycomb-trace")
				ctx, tr = trace.NewTrace(ctx, beelineHeader)
			} else {
				prop := parserHook(ctx)
				ctx, tr = trace.NewTraceFromPropagationContext(ctx, prop)
			}
		} else {
			ctx, tr = trace.NewTrace(ctx, "")
		}
		span = tr.GetRootSpan()
	} else {
		// we had a parent! let's make a new child for this handler
		ctx, span = span.CreateChild(ctx)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		span.AddField("request.content_type", md["content-type"][0])
	}
	return ctx, span
}

// WrappedUnaryServerInterceptor is a function that takes a GRPCIncomingConfig and returns a
// GRPC interceptor.
func WrappedUnaryServerInterceptor(cfg config.GRPCIncomingConfig) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		handlerName := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()

		ctx, span := startSpanOrTraceFromUnaryGRPC(ctx, info, cfg.GRPCParserHook)

		span.AddField("handler.name", handlerName)
		span.AddField("name", handlerName)
		defer span.Send()

		resp, err := handler(ctx, req)
		return resp, err
	}
}
