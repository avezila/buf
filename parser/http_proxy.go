package parser

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
)

func HTTPGetProxy(ctx echo.Context, pctx *Context) error {
	proxy := []Proxy{}
	err := pctx.Db.
		Model(&proxy).
		Order("db_add_time DESC").
		Select()
	if err != nil {
		return ctx.JSON(500, struct{ Error string }{fmt.Sprintf("%+v", err)})
	}
	return ctx.JSON(200, proxy)
}

type PostProxyReq struct {
	Proxy []Proxy
}

func HTTPPostProxy(ctx echo.Context, pctx *Context) error {
	req := []string{}
	proxy := []Proxy{}
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, struct{ Error string }{fmt.Sprintf("%+v", err)})
	}
	for i := range req {
		proxy = append(proxy, Proxy{IP: &req[i]})
	}
	ret, err := pctx.Db.Model(&proxy).
		OnConflict("(ip) DO NOTHING").
		Insert()
	if err != nil {
		return ctx.JSON(500, struct{ Error string }{fmt.Sprintf("%+v", err)})
	}
	return ctx.JSON(200, struct{ RowsAffected int }{
		ret.RowsAffected(),
	})
}
