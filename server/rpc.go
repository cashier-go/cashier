package server

import (
	"log"
	"net"

	"golang.org/x/net/context"

	"golang.org/x/oauth2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/proto"
)

type rpcServer struct{}

type key int

const usernameKey key = 0
const principalsKey key = 1

func (s *rpcServer) Sign(ctx context.Context, req *proto.SignRequest) (*proto.SignResponse, error) {
	username, ok := ctx.Value(usernameKey).(string)
	if !ok {
		return nil, grpc.Errorf(codes.InvalidArgument, "Error reading username")
	}
	principals, ok := ctx.Value(principalsKey).([]string)
	if !ok {
		return nil, grpc.Errorf(codes.InvalidArgument, "Error reading principals")
	}
	cert, err := keysigner.SignUserKeyFromRPC(req, username, principals)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, err.Error())
	}
	if err := certstore.SetCert(cert); err != nil {
		log.Printf("Error recording cert: %v", err)
	}
	resp := &proto.SignResponse{
		Cert: lib.GetPublicKey(cert),
	}
	return resp, nil
}

func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	md, ok := metadata.FromContext(ctx)
	if !ok {
		return nil, grpc.Errorf(codes.Unauthenticated, "request not authenticated")
	}
	switch md["security"][0] {
	case "authorization":
		token := &oauth2.Token{
			AccessToken: md["payload"][0],
		}
		if !authprovider.Valid(token) {
			return nil, grpc.Errorf(codes.PermissionDenied, "access denied")
		}
		ctx = context.WithValue(ctx, usernameKey, authprovider.Username(token))
		ctx = context.WithValue(ctx, principalsKey, authprovider.Principals(token))
		authprovider.Revoke(token)
	default:
		return nil, grpc.Errorf(codes.InvalidArgument, "unknown argument")
	}
	return handler(ctx, req)
}

func runGRPCServer(l net.Listener) {
	serv := grpc.NewServer(grpc.UnaryInterceptor(authInterceptor))
	proto.RegisterSignerServer(serv, &rpcServer{})
	serv.Serve(l)
}
