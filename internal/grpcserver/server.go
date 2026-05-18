package grpcserver

import (
	"context"
	"errors"
	"strings"

	"github.com/mmeow0/meow-shortener/internal/facade"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"github.com/mmeow0/meow-shortener/internal/pb"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	pb.UnimplementedShortenerServiceServer
	facade *facade.URLFacade
	logger *zap.Logger
}

func New(facade *facade.URLFacade, logger *zap.Logger) *Server {
	return &Server{
		facade: facade,
		logger: logger,
	}
}

func (s *Server) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	if strings.TrimSpace(req.GetUrl()) == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}

	userID := middleware.GetUserID(ctx, s.logger)
	shortURL, err := s.facade.ShortenURL(ctx, req.GetUrl(), userID)
	if err != nil {
		var conflictErr *facade.ConflictError
		if errors.As(err, &conflictErr) {
			return &pb.URLShortenResponse{Result: conflictErr.Result}, nil
		}

		s.logger.Error("failed to shorten url", zap.String("url", req.GetUrl()), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to shorten url")
	}

	return &pb.URLShortenResponse{Result: shortURL}, nil
}

func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	if strings.TrimSpace(req.GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	userID := middleware.GetUserID(ctx, s.logger)
	originalURL, err := s.facade.ExpandURL(ctx, req.GetId(), userID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, status.Error(codes.NotFound, "url not found")
		case errors.Is(err, repository.ErrDeleted):
			return nil, status.Error(codes.FailedPrecondition, "url has been deleted")
		default:
			s.logger.Error("failed to expand url", zap.String("id", req.GetId()), zap.Error(err))
			return nil, status.Error(codes.Internal, "failed to expand url")
		}
	}

	return &pb.URLExpandResponse{Result: originalURL}, nil
}

func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID := middleware.GetUserID(ctx, s.logger)
	urls, err := s.facade.ListUserURLs(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list user urls", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to list user urls")
	}

	response := &pb.UserURLsResponse{
		Url: make([]*pb.URLData, 0, len(urls)),
	}
	for _, item := range urls {
		response.Url = append(response.Url, &pb.URLData{
			ShortUrl:    item.ShortURL,
			OriginalUrl: item.OriginalURL,
		})
	}

	return response, nil
}
