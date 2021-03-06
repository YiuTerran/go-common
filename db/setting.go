package db

import (
	"github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"
	"sync"
)

var once sync.Once

func UseSnakeCase(db *sqlx.DB) {
	once.Do(func() {
		//ćšć±ææ
		sqlbuilder.DefaultFieldMapper = sqlbuilder.SnakeCaseMapper
	})
	db.MapperFunc(sqlbuilder.SnakeCaseMapper)
}
