package gmailx

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GmailRPC struct {
	once sync.Once

	Client       *http.Client
	Ctx          context.Context
	srv          *gmail.Service
	closedReason error
	closed       atomic.Bool
}

type ListMessageOpt struct {
}

func (grpc *GmailRPC) ListMessage(opts ListMessageOpt) error {
	srv, err := grpc.gmailSrv()
	if err != nil {
		return err
	}

	res, err := srv.Users.Messages.List("me").Do()
	if err != nil {
		return err
	}

	fmt.Printf("Status code: %d\n", res.HTTPStatusCode)
	fmt.Printf("Estimated Count: %d\n", res.ResultSizeEstimate)
	for i, message := range res.Messages {
		fmt.Printf("Message #%d: id %s\n", i, message.Id)
		res, err := srv.Users.Messages.Get("me", message.Id).Do()
		if err != nil {
			fmt.Printf("Message #%d: error %s\n", i, err.Error())
			continue
		}
		fmt.Printf("Message #%d: raw %s\n", i, res.Snippet)
	}

	return nil
}

func (grpc *GmailRPC) gmailSrv() (*gmail.Service, error) {
	grpc.once.Do(func() {
		srv, err := gmail.NewService(grpc.Ctx, option.WithHTTPClient(grpc.Client))
		if err != nil {
			grpc.closedReason = err
			grpc.closed.Store(true)
		}

		grpc.srv = srv
	})

	return grpc.srv, grpc.closedReason
}
