// Package foo holds the hand-written implementation of FooService, kept
// separate from the generated foov1connect package so generated and
// hand-written code stay cleanly divided.
package foo

import (
	"context"

	connect "connectrpc.com/connect"
	foov1 "github.com/sethlowie/dinnerwise/internal/foo/v1"
	"github.com/sethlowie/dinnerwise/internal/foo/v1/foov1connect"
)

// Service is a placeholder implementation of FooService used to prove the
// React → Connect → Go loop end-to-end.
type Service struct{}

// NewService returns a FooServiceHandler ready to mount via
// foov1connect.NewFooServiceHandler.
func NewService() foov1connect.FooServiceHandler {
	return &Service{}
}

func (s *Service) GetFoo(
	ctx context.Context,
	req *connect.Request[foov1.GetFooRequest],
) (
	*connect.Response[foov1.GetFooResponse],
	error,
) {
	return connect.NewResponse(&foov1.GetFooResponse{
		Data: &foov1.Foo{
			Foo: "hello from foo " + req.Msg.GetId(),
		},
	}), nil
}
