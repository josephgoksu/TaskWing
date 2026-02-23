package grpc

import (
	"context"
	"log"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// recoveryUnaryInterceptor catches panics in unary RPCs and returns INTERNAL.
func recoveryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] %s: %v\n%s", info.FullMethod, r, debug.Stack())
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// recoveryStreamInterceptor catches panics in streaming RPCs and returns INTERNAL.
func recoveryStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] %s: %v\n%s", info.FullMethod, r, debug.Stack())
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(srv, ss)
	}
}

// loggingUnaryInterceptor logs RPC method and duration.
func loggingUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err)
		log.Printf("[gRPC] %s %s %s", info.FullMethod, code, time.Since(start).Round(time.Millisecond))
		return resp, err
	}
}

// loggingStreamInterceptor logs streaming RPC method and duration.
func loggingStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		code := status.Code(err)
		log.Printf("[gRPC] %s %s %s", info.FullMethod, code, time.Since(start).Round(time.Millisecond))
		return err
	}
}
