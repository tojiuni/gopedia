package nlpworker

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "gopedia/core/proto/gen/go"
)

type Client struct {
	addr string
}

func New(addr string) *Client {
	return &Client{addr: addr}
}

func (c *Client) ProcessL2(ctx context.Context, req *pb.NLPRequest) (*pb.NLPResponse, error) {
	if c == nil || c.addr == "" {
		return nil, fmt.Errorf("nlp worker addr not set")
	}
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewNLPWorkerClient(conn)
	callCtx, cancel2 := context.WithTimeout(ctx, 10*time.Second)
	defer cancel2()
	return client.ProcessL2(callCtx, req)
}

