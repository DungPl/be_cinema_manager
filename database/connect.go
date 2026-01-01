package database

import (
	"cinema_manager/config"
	"cinema_manager/model"
	"fmt"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	var err error
	p := config.Config("DB_PORT")
	port, err := strconv.ParseUint(p, 10, 32)

	if err != nil {
		panic("failed to parse database port")
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", config.Config("DB_HOST"), port, config.Config("DB_USER"), config.Config("DB_PASSWORD"), config.Config("DB_NAME"))
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		panic("failed to connect database")
	}

	fmt.Println("Connection Opened to Database")
	DB.AutoMigrate(
		&model.Account{},
		&model.Staff{},
		&model.Customer{},
		&model.Cinema{},
		&model.Format{},
		&model.RoomFormat{},
		&model.MovieFormat{},
		&model.Address{},
		&model.Movie{},
		&model.MoviePoster{},
		&model.MovieTrailer{},
		&model.Room{},
		&model.Holiday{},
		&model.ScheduleTemplate{},
		&model.Showtime{},
		&model.Ticket{},
		&model.SeatType{},
		&model.Seat{},
		&model.Promotion{},
		&model.ShowtimeSeat{},
		&model.Order{},
		&model.PromotionCondition{},
		&model.PromotionUsage{},
		&model.PasswordResetToken{},
	)
	fmt.Println("Database Migrated")

	// khởi tạo dữ liệu
	SeedData(DB)
}
