package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/1suo/hiddify-tui/gen/hcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type connectTestServer struct {
	hcore.UnimplementedCoreServer
	startCalls   int
	restartCalls int
}

func (s *connectTestServer) Start(context.Context, *hcore.StartRequest) (*hcore.CoreInfoResponse, error) {
	s.startCalls++
	return &hcore.CoreInfoResponse{
		CoreState:   hcore.CoreStates_STARTED,
		MessageType: hcore.MessageType_ALREADY_STARTED,
	}, nil
}

func (s *connectTestServer) Restart(context.Context, *hcore.StartRequest) (*hcore.CoreInfoResponse, error) {
	s.restartCalls++
	return &hcore.CoreInfoResponse{CoreState: hcore.CoreStates_STARTED}, nil
}

func TestConnectReplacesBootstrapConfig(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	coreServer := &connectTestServer{}
	hcore.RegisterCoreServer(server, coreServer)
	go server.Serve(listener)
	defer server.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(ctx, "passthrough:///bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	coreClient := &GRPCClient{conn: connection, api: hcore.NewCoreClient(connection)}
	defer coreClient.Close()
	if err := coreClient.Connect(ctx, `{"outbounds":[]}`, "profile"); err != nil {
		t.Fatal(err)
	}
	if coreServer.startCalls != 1 || coreServer.restartCalls != 1 {
		t.Fatalf("Start calls = %d, Restart calls = %d; want 1 each", coreServer.startCalls, coreServer.restartCalls)
	}
}
