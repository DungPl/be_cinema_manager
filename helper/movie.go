// helper/movie_helper.go
package helper

import (
	"cinema_manager/database"
	"cinema_manager/model"
	"log"
	"time"

	"github.com/go-co-op/gocron/v2"
)

var movieScheduler gocron.Scheduler

func AutoUpdateMovieStatus() {
	log.Println("[CRON] AutoUpdateMovieStatus triggered")

	db := database.DB
	loc := time.FixedZone("ICT", 7*3600)
	today := time.Now().In(loc).Truncate(24 * time.Hour)

	var movies []model.Movie
	if err := db.Find(&movies).Error; err != nil {
		log.Printf("Lỗi khi quét phim: %v", err)
		return
	}

	for _, movie := range movies {
		updated := false

		releaseDate := movie.DateRelease.Time.In(loc).Truncate(24 * time.Hour)

		// Từ COMING_SOON → NOW_SHOWING khi đến hoặc qua ngày khởi chiếu
		if (today.Equal(releaseDate) || today.After(releaseDate)) && movie.StatusMovie == "COMING_SOON" {
			movie.StatusMovie = "NOW_SHOWING"
			updated = true
		}

		// Từ NOW_SHOWING → ENDED khi qua ngày kết thúc
		if movie.DateEnd != nil {
			endDate := movie.DateEnd.Time.In(loc).Truncate(24 * time.Hour)
			if today.After(endDate) && movie.StatusMovie == "NOW_SHOWING" {
				movie.StatusMovie = "ENDED"
				updated = true
			}
		}

		if updated {
			if err := db.Save(&movie).Error; err != nil {
				log.Printf("Lỗi cập nhật trạng thái phim '%s': %v", movie.Title, err)
			} else {
				log.Printf("Cập nhật trạng thái phim '%s' → %s", movie.Title, movie.StatusMovie)
			}
		}
	}
}

func StartMovieStatusScheduler() {
	s, err := gocron.NewScheduler(
		gocron.WithLocation(time.FixedZone("ICT", 7*3600)),
	)
	if err != nil {
		log.Fatal(err)
	}

	movieScheduler = s

	_, err = s.NewJob(
		gocron.DailyJob(
			1,
			gocron.NewAtTimes(
				gocron.NewAtTime(0, 5, 0),
			),
		),
		gocron.NewTask(AutoUpdateMovieStatus),
	)
	if err != nil {
		log.Fatal(err)
	}

	s.Start()
	log.Println("✅ Movie status scheduler started (00:05 ICT)")
}
