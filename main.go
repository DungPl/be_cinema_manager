package main

import (
	"cinema_manager/database"
	"cinema_manager/handler"
	"cinema_manager/helper"
	"cinema_manager/router"
	"cinema_manager/utils"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	app := fiber.New(fiber.Config{
		BodyLimit: 100 * 1024 * 1024, // ✅ cho phép upload tối đa 100MB
	})
	helper.StartShowtimeScheduler()
	defer helper.StopShowtimeScheduler()
	handler.StartExpireSeatWorker()
	helper.StartMovieStatusScheduler()
	go func() {
		ticker := time.NewTicker(1 * time.Minute) // Chạy mỗi 1 phút
		defer ticker.Stop()

		for {
			<-ticker.C
			handler.ExpireTickets()
		}
	}()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5173/",
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Authorization, Accept",
		AllowCredentials: true,
		ExposeHeaders:    "Set-Cookie",
		MaxAge:           600,
	}))

	database.ConnectDB()

	router.SetupRoutes(app)
	utils.SetupVietmapRoutes(app)
	log.Fatal(app.Listen(":8002"))
}
