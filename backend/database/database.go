package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/joho/godotenv"
)

var DB *sql.DB

func Connect() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal(fmt.Errorf("unable to connect to database: %w", err))
	}

	// --- NEW: CẤU HÌNH POOLING ---
	// Giới hạn số kết nối tối đa (tránh panic: too many clients)
	db.SetMaxOpenConns(25)

	// Số kết nối rảnh rỗi giữ lại để tái sử dụng
	db.SetMaxIdleConns(5)

	// Thời gian sống tối đa của 1 kết nối (tránh kết nối bị treo ảo)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatal(fmt.Errorf("unable to ping database: %w", err))
	}

	DB = db
	log.Println("Database connection successful")
}
