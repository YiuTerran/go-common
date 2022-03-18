package db

import (
	"github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"
	"sync"
)

var once sync.Once

func UseSnakeCase(db *sqlx.DB) {
	once.Do(func() {
		//全局有效
		sqlbuilder.DefaultFieldMapper = sqlbuilder.SnakeCaseMapper
	})
	db.MapperFunc(sqlbuilder.SnakeCaseMapper)
}
