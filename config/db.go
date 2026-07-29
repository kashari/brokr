package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

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

		// Tune the underlying database/sql pool. With the actor-per-instance
		// dispatcher, many goroutines hit the DB in parallel; the stdlib default
		// of 2 idle connections would serialize them into a bottleneck.
		sqlDB, err := Db.DB()
		if err != nil {
			golog.Error("Failed to access underlying sql.DB: {}", err.Error())
			panic("failed to access sql.DB: " + err.Error())
		}
		sqlDB.SetMaxOpenConns(getEnvInt("DB_MAX_OPEN_CONNS", 40))
		sqlDB.SetMaxIdleConns(getEnvInt("DB_MAX_IDLE_CONNS", 20))
		sqlDB.SetConnMaxLifetime(time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_SEC", 1800)) * time.Second)
		sqlDB.SetConnMaxIdleTime(time.Duration(getEnvInt("DB_CONN_MAX_IDLE_SEC", 300)) * time.Second)

		golog.Info("Database connection established successfully")
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
		golog.Error("Invalid int for {} ({}), using default {}", key, value, defaultValue)
	}
	return defaultValue
}
