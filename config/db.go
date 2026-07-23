package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/kashari/golog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	Db   *gorm.DB
	once sync.Once
)

func InitDB() {
	once.Do(func() {
		// Get DB config from environment variables with defaults
		host := getEnv("DB_HOST", "127.0.0.1")
		port := getEnv("DB_PORT", "5436")
		user := getEnv("DB_USER", "misen")
		password := getEnv("DB_PASSWORD", "root")
		dbname := getEnv("DB_NAME", "workflow")
		sslmode := getEnv("DB_SSLMODE", "disable")

		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
			host, user, password, dbname, port, sslmode,
		)

		golog.Info("Connecting to database at {}:{}/{}", host, port, dbname)

		var err error
		Db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			golog.Error("Failed to connect to database: {}", err.Error())
			panic("failed to connect to database: " + err.Error())
		}

		golog.Info("Database connection established successfully")
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
