package parser

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

func HTTPPostParsePage(ctx echo.Context, pctx *Context) error {
	urls := []string{}

	if err := ctx.Bind(&urls); err != nil {
		return ctx.JSON(http.StatusBadRequest, struct{ Error string }{fmt.Sprintf("%+v", errors.Wrap(err, "Failed read request"))})
	}
	for _, url := range urls {
		if err := RequestParsePage(pctx, url, 100); err != nil {
			return ctx.JSON(http.StatusInternalServerError, struct{ Error string }{fmt.Sprintf("%+v", errors.Wrap(err, "Failed queue request for parse page"))})
		}
	}
	return ctx.JSON(200, struct{}{})
}
