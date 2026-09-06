package database

import (
	"fmt"
	"log"
	"net/url"

	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

// Built through url.URL so the credentials get userinfo escaping, not query escaping:
// QueryEscape turns a space into "+", which a driver reads back as a literal plus.
// opts is appended raw, since it arrives already encoded from the config file.
func sqlServerDSN(host string, port int, user string, password string, database string, opts string) string {
	dsn := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(user, password),
		Host:     fmt.Sprintf("%s:%d", host, port),
		RawQuery: url.Values{"database": {database}}.Encode(),
	}
	if opts != "" {
		dsn.RawQuery += "&" + opts
	}
	return dsn.String()
}

// NewSQLServer opens a pooled GORM connection to a SQL Server database and pings it.
func NewSQLServer(
	host string, port int, user string, password string, database string, opts string, pool PoolConfig,
) (*gorm.DB, error) {
	if !hasTLS(opts, "encrypt=true", "encrypt=strict") {
		log.Printf("warning: SQL Server connection to %s:%d is not encrypted; "+
			"credentials and query data cross the network in cleartext (opts=%q)\n", host, port, opts)
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
