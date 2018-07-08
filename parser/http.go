package parser

import (
	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

func StartServer(pctx *Context) error {
	server := echo.New()
	server.GET("/api/proxy", func(ctx echo.Context) error {
		return HTTPGetProxy(ctx, pctx)
	})
	server.POST("/api/proxy", func(ctx echo.Context) error {
		return HTTPPostProxy(ctx, pctx)
	})
	server.POST("/api/parse_page", func(ctx echo.Context) error {
		return HTTPPostParsePage(ctx, pctx)
	})
	if err := server.Start(ENV_LOCAL_IP + ":" + ENV_SERVICE_PORT); err != nil {
		return errors.Wrap(err, "Failed start server in parser.StartServer")
	}
	return nil
}
