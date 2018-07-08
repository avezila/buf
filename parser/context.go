package parser

import (
	"context"

	"github.com/globalsign/mgo"
	"github.com/go-pg/pg"
	"github.com/jackc/pgx"
)

type Context struct {
	Ctx context.Context
	MS  *mgo.Session
	Db  *pg.DB
	Dbx *pgx.ConnPool
}
