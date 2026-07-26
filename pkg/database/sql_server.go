package database

import (
	"fmt"
	"log"
	"net/url"

	"github.com/supernurture/go-template/pkg/util"

	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

func sqlServerDSN(host string, port int, user string, password string, database string, opts string) string {
	return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s%s",
		user, url.QueryEscape(password), host, port, database,
		util.Ternary(opts != "", fmt.Sprintf("&%s", opts), ""))
}

// NewSQLServer opens a pooled GORM connection to a SQL Server database and pings it.
func NewSQLServer(host string, port int, user string, password string, database string, opts string, pool PoolConfig) (*gorm.DB, error) {
	if !hasTLS(opts, "encrypt=true", "encrypt=strict") {
		log.Printf("warning: SQL Server connection to %s:%d is not encrypted; credentials and query data cross the network in cleartext (opts=%q)\n", host, port, opts)
	}

	db, err := gormOpen(
		sqlserver.Open(sqlServerDSN(host, port, user, password, database, opts)),
		&gorm.Config{Logger: gormLogger},
	)
	if err != nil {
		return nil, err
	}

	if err := configurePool(db, pool); err != nil {
		return nil, err
	}

	if err := ping(db); err != nil {
		return nil, err
	}

	log.Printf("connected to SQL Server database %q at %s:%d\n", database, host, port)
	return db, nil
}
