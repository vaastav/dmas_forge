package tools

import (
	"context"
	"fmt"
)

type imageOutputKey struct{}

func WithImageOutput(ctx context.Context, dst *[]byte) context.Context {
	return context.WithValue(ctx, imageOutputKey{}, dst)
}

func imageOutputFromContext(ctx context.Context) (*[]byte, error) {
	dst, ok := ctx.Value(imageOutputKey{}).(*[]byte)
	if !ok || dst == nil {
		return nil, fmt.Errorf("image output buffer missing from context")
	}
	return dst, nil
}
