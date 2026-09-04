package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	Mysql struct {
		DataSource string
	}
	Redis         redis.RedisConf
	PublicBaseUrl string `json:",default=http://localhost:5173"`
	UploadDir     string `json:",default=./data/uploads"`
	Wechat        struct {
		AppId     string `json:",optional"`
		AppSecret string `json:",optional"`
	} `json:",optional"`
}
