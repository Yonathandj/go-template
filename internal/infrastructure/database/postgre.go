package database

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
)

func PostgreInit(ctx context.Context, databaseName string) {
	var (
		username = viper.GetString(fmt.Sprintf("database.postgre.%s.username", databaseName))
		password = viper.GetString(fmt.Sprintf("database.postgre.%s.password", databaseName))
		host     = viper.Get(fmt.Sprintf("database.postgre.%s.host", databaseName))
		port     = viper.GetInt(fmt.Sprintf("database.postgre.%s.port", databaseName))
		database = viper.GetString(fmt.Sprintf("database.postgre.%s.database", databaseName))
	)
}
