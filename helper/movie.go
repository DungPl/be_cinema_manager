// helper/movie_helper.go
package helper

import (
	"cinema_manager/database"
	"cinema_manager/model"
	"log"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// AutoUpdateMovieStatus chạy hàng ngày để cập nhật trạng thái phim
func AutoUpdateMovieStatus() {
	db := database.DB
	loc := time.FixedZone("ICT", 7*3600)
	today := time.Now().In(loc).Truncate(24 * time.Hour)

	var movies []model.Movie
	if err := db.Find(&movies).Error; err != nil {
		log.Printf("Lỗi khi quét phim: %v", err)
		return
	}

	updatedToNow := 0
	updatedToEnded := 0

	for _, movie := range movies {
		updated := false

		// 1. COMING_SOON → NOW_SHOWING (ĐÚNG NGÀY PHÁT HÀNH)
		releaseDate := movie.DateRelease.Time.Truncate(24 * time.Hour)
		if releaseDate.Equal(today) && movie.StatusMovie == "COMING_SOON" {
			movie.StatusMovie = "NOW_SHOWING"
			updated = true
			updatedToNow++
		}

		// 2. NOW_SHOWING → ENDED (date_end < hôm nay)
		if movie.DateEnd != nil {
			endDate := movie.DateEnd.Time.Truncate(24 * time.Hour)
			if endDate.Before(today) && movie.StatusMovie == "NOW_SHOWING" {
				movie.StatusMovie = "ENDED"
				updated = true
				updatedToEnded++
			}
		}

		if updated {
			if err := db.Save(&movie).Error; err != nil {
				log.Printf("Lỗi cập nhật phim %s: %v", movie.Title, err)
			}
		}
	}

	log.Printf(
		"Cron movie status: %d → NOW_SHOWING, %d → ENDED",
		updatedToNow,
		updatedToEnded,
	)
}

func StartMovieStatusScheduler() {
	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal("Lỗi khởi tạo scheduler: ", err)
	}

	// Chạy lúc 00:05 sáng hàng ngày (giờ Việt Nam)
	_, err = s.NewJob(
		gocron.DailyJob(
			1,
			gocron.NewAtTimes(
				gocron.NewAtTime(0, 5, 0), // 00:05:00
			),
		),
		gocron.NewTask(AutoUpdateMovieStatus),
	)
	if err != nil {
		log.Fatal("Lỗi tạo job cập nhật trạng thái phim: ", err)
	}

	s.Start()
	log.Println("Scheduler cập nhật trạng thái phim đã khởi động (chạy lúc 00:05 hàng ngày)")
}
