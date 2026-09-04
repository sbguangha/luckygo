package main

import (
	"context"
	"flag"
	"time"

	"luckygo/internal/service"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type Config struct {
	Log   logx.LogConf
	Mysql struct {
		DataSource string
	}
	Redis redis.RedisConf
}

var configFile = flag.String("f", "etc/worker.yaml", "config")

func main() {
	flag.Parse()
	var c Config
	conf.MustLoad(*configFile, &c)
	logx.MustSetup(c.Log)
	rdb := redis.MustNewRedis(c.Redis)
	app := service.New(service.Conf{JWTSecret: "unused", JWTExpire: 1, PublicBase: ""}, c.Mysql.DataSource, rdb)
	logx.Info("luckygo worker started")
	tk := time.NewTicker(time.Second)
	defer tk.Stop()
	for range tk.C {
		app.HandleDueJobs(context.Background())
	}
}
