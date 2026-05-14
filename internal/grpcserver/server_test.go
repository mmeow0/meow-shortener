package grpcserver

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mmeow0/meow-shortener/internal/facade"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"github.com/mmeow0/meow-shortener/internal/pb"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestShortenerServiceGRPCFlow(t *testing.T) {
	t.Helper()

	const secretKey = "test-secret"
	const bufSize = 1024 * 1024

	repo := repository.NewInMemoryURLRepository()
	svc := service.NewURLService(repo)
	urlFacade := facade.NewURLFacade(svc, "http://localhost:8080", nil)
	authenticator := middleware.NewAuthenticator(secretKey, zap.NewNop())

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.GRPCAuthInterceptor(authenticator),
			middleware.TimeoutUnaryInterceptor(5*time.Second),
		),
	)
	pb.RegisterShortenerServiceServer(grpcSrv, New(urlFacade, zap.NewNop()))

	listener := bufconn.Listen(bufSize)
	defer listener.Close()

	go func() {
		_ = grpcSrv.Serve(listener)
	}()
	defer grpcSrv.GracefulStop()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}
	defer conn.Close()

	client := pb.NewShortenerServiceClient(conn)

	var shortenHeader metadata.MD
	shortenResponse, err := client.ShortenURL(
		context.Background(),
		&pb.URLShortenRequest{Url: "https://example.com/alpha"},
		grpc.Header(&shortenHeader),
	)
	if err != nil {
		t.Fatalf("ShortenURL failed: %v", err)
	}
	if !strings.HasPrefix(shortenResponse.GetResult(), "http://localhost:8080/") {
		t.Fatalf("unexpected shorten result: %s", shortenResponse.GetResult())
	}

	authValues := shortenHeader.Get("authorization")
	if len(authValues) == 0 || !strings.HasPrefix(authValues[0], "Bearer ") {
		t.Fatalf("expected authorization header, got: %v", authValues)
	}

	shortID := strings.TrimPrefix(shortenResponse.GetResult(), "http://localhost:8080/")
	callCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", authValues[0]))

	expandResponse, err := client.ExpandURL(callCtx, &pb.URLExpandRequest{Id: shortID})
	if err != nil {
		t.Fatalf("ExpandURL failed: %v", err)
	}
	if got := expandResponse.GetResult(); got != "https://example.com/alpha" {
		t.Fatalf("unexpected expand result: %s", got)
	}

	listResponse, err := client.ListUserURLs(callCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListUserURLs failed: %v", err)
	}
	if len(listResponse.GetUrl()) != 1 {
		t.Fatalf("expected 1 user url, got %d", len(listResponse.GetUrl()))
	}
	if got := listResponse.GetUrl()[0].GetOriginalUrl(); got != "https://example.com/alpha" {
		t.Fatalf("unexpected original url: %s", got)
	}
	if got := listResponse.GetUrl()[0].GetShortUrl(); got != shortenResponse.GetResult() {
		t.Fatalf("unexpected short url: %s", got)
	}
}
