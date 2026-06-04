package sharkgrpc

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/singleflight"

	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

var connections sync.Map

type connection struct {
	conn     *grpc.ClientConn
	resolver *manual.Resolver
	addrs    atomic.Value
}

type RpcServer struct {
	ctx     context.Context
	Server  *grpc.Server
	logger  *zap.Logger
	redis   *redis.ClusterClient
	port    int
	sg      singleflight.Group
	project string
}

func New(ctx context.Context, project string, redis *redis.ClusterClient, logger *zap.Logger, port int) *RpcServer {
	s := &RpcServer{
		ctx:    ctx,
		Server: grpc.NewServer(),
		logger: logger,
		redis:  redis,
		port:   port,
	}
	go s.run()
	return s
}

func (s *RpcServer) run() {
	time.Sleep(time.Second)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", s.port))
	if err != nil {
		s.logger.Error("failed to listen", zap.Error(err))
	}
	err = s.Server.Serve(listener)
	if err != nil {
		s.logger.Error("failed to serve", zap.Error(err))
	}
}

func (s *RpcServer) updateResolver(name string) {
	v, ok := connections.Load(name)
	if !ok {
		return
	}
	c := v.(*connection)
	for {
		time.Sleep(5 * time.Second)
		addr, err := s.redis.Get(s.ctx, s.redisGrpcHost(name)).Result()
		if err != nil {
			s.logger.Error("failed to get grpc address from redis", zap.String("name", name), zap.Error(err))
			continue
		}
		addr = strings.TrimSpace(addr)
		if addr == "" {
			s.logger.Warn("grpc address is empty", zap.String("name", name))
			continue
		}
		addrs := []string{}
		err = json.Unmarshal([]byte(addr), &addrs)
		if err != nil {
			s.logger.Error("failed to unmarshal grpc address", zap.String("name", name), zap.String("addr", addr), zap.Error(err))
			continue
		}
		if len(addrs) == 0 {
			s.logger.Warn("grpc address is empty", zap.String("name", name))
		}
		var oldAddrs []string
		if v := c.addrs.Load(); v != nil {
			oldAddrs = append([]string(nil), v.([]string)...)
		}
		sort.Strings(addrs)
		sort.Strings(oldAddrs)
		if len(addrs) == len(oldAddrs) {
			same := true
			for i := range addrs {
				if addrs[i] != oldAddrs[i] {
					same = false
					break
				}
			}
			if same {
				continue
			}
		}
		var resolverAddrs []resolver.Address = make([]resolver.Address, 0, len(addrs))
		for _, addr := range addrs {
			resolverAddrs = append(resolverAddrs, resolver.Address{Addr: addr})
		}
		c.resolver.UpdateState(resolver.State{Addresses: resolverAddrs})
		c.addrs.Store(append([]string{}, addrs...))
	}
}

func (s *RpcServer) redisGrpcHost(name string) string {
	return fmt.Sprintf("%v:grpc:%v", s.project, name)
}

func (s *RpcServer) GetRpcConnection(name string) (*grpc.ClientConn, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("GetRpcConnection redis未初始化")
	}
	conn, err, _ := s.sg.Do("grpc-conn-"+name, func() (any, error) {
		v, ok := connections.Load(name)
		if ok {
			return v.(*connection), nil
		} else {
			addr, err := s.redis.Get(s.ctx, s.redisGrpcHost(name)).Result()
			if err != nil && err != redis.Nil {
				return nil, err
			}
			addr = strings.TrimSpace(addr)
			if addr == "" {
				s.redis.Set(s.ctx, s.redisGrpcHost(name), "[]", 0)
				return nil, fmt.Errorf("grpc地址未配置")
			}
			addrs := []string{}
			err = json.Unmarshal([]byte(addr), &addrs)
			if err != nil {
				return nil, fmt.Errorf("grpc地址配置错误")
			}
			if len(addrs) == 0 {
				return nil, fmt.Errorf("grpc地址未配置")
			}
			sort.Strings(addrs)
			c := &connection{
				resolver: manual.NewBuilderWithScheme("custom"),
			}
			c.addrs.Store(append([]string{}, addrs...))
			var resolverAddrs []resolver.Address = make([]resolver.Address, 0, len(addrs))
			for _, addr := range addrs {
				resolverAddrs = append(resolverAddrs, resolver.Address{Addr: addr})
			}
			c.resolver.UpdateState(resolver.State{Addresses: resolverAddrs})
			conn, err := grpc.NewClient(
				"custom:///svc",
				grpc.WithResolvers(c.resolver),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithConnectParams(grpc.ConnectParams{
					Backoff: backoff.Config{
						BaseDelay:  100 * time.Millisecond,
						Multiplier: 1.6,
						MaxDelay:   3 * time.Second,
					},
					MinConnectTimeout: 1 * time.Second,
				}),
				grpc.WithDefaultServiceConfig(`{
  "loadBalancingPolicy":"round_robin",
  "methodConfig":[{
    "name":[{"service":"your.service"}],
    "retryPolicy":{
      "MaxAttempts":3,
      "InitialBackoff":"0.05s",
      "MaxBackoff":"0.5s",
      "BackoffMultiplier":1.6,
      "RetryableStatusCodes":[
        "UNAVAILABLE",
        "RESOURCE_EXHAUSTED"
      ]
    }
  }]
}`),
			)
			c.conn = conn
			connections.Store(name, c)
			go s.updateResolver(name)
			return conn, err
		}
	})
	if err != nil {
		return nil, err
	}
	return conn.(*grpc.ClientConn), nil
}
