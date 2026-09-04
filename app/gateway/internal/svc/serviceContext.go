package svc

import (
	"luckygo/app/gateway/internal/config"
	"luckygo/internal/service"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type ServiceContext struct {
	Config config.Config
	App    *service.App
}

func NewServiceContext(c config.Config) *ServiceContext {
	rdb := redis.MustNewRedis(c.Redis)
	app := service.New(service.Conf{
		JWTSecret:  c.Auth.AccessSecret,
		JWTExpire:  c.Auth.AccessExpire,
		PublicBase: c.PublicBaseUrl,
	}, c.Mysql.DataSource, rdb)
	return &ServiceContext{Config: c, App: app}
}
