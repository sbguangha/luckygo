package main

import (
	"flag"
	"net/http"

	"luckygo/app/gateway/internal/config"
	"luckygo/app/gateway/internal/handler"
	"luckygo/app/gateway/internal/svc"
	"luckygo/internal/httperr"
	"luckygo/internal/xerr"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/luckygo-api.yaml", "the config file")

func main() {
	flag.Parse()
	httperr.Register()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf, rest.WithCors())
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)
	server.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/healthz",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			if err := ctx.App.DB.Ping(r.Context()); err != nil {
				logx.Errorf("health mysql: %v", err)
				httpx.ErrorCtx(r.Context(), w, xerr.Internal())
				return
			}
			httpx.OkJsonCtx(r.Context(), w, map[string]string{"status": "ok"})
		},
	})

	logx.Infof("luckygo api listening on %s:%d", c.Host, c.Port)
	server.Start()
}
