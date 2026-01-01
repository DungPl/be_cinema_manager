package router

import (
	"cinema_manager/handler"
	"cinema_manager/middleware"
	"cinema_manager/validate"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func SetupRoutes(app *fiber.App) {
	api := app.Group("/api", logger.New())
	v1 := api.Group("/v1", logger.New())

	auth := v1.Group("/auth")
	auth.Post("/login", handler.Login)
	auth.Post("/refresh-token", handler.RefreshToken)

	account := v1.Group("/account", logger.New())
	account.Get("/", middleware.Protected(), handler.GetAccounts)
	account.Get("/me", middleware.Protected(), handler.Me)
	account.Post("/", middleware.Protected(), validate.CreateAccount(), handler.CreateAccount)
	account.Post("/change-password", middleware.Protected(), validate.AdminChangePassword(), handler.AdminChangePassword)
	account.Patch("/:accountId/active", middleware.Protected(), validate.ActiveAccount(), handler.ActiveAccount)
	account.Put("/:accountId/cinema", middleware.Protected(), validate.UpdateManagerCinema(), handler.UpdateManagerCinema)

	staff := v1.Group("/staff", logger.New())
	staff.Get("/", middleware.Protected(), handler.GetStaffs)
	staff.Post("/seats/hold/:code", middleware.Protected(), handler.HoldSeatForStaff)
	staff.Get("/seats/held/:code", middleware.Protected(), handler.GetHeldSeatsForStaff)
	staff.Post("/seats/release/:code", middleware.Protected(), handler.ReleaseSeatForStaff)
	staff.Post("/ticket/create/:code", middleware.Protected(), handler.CreateTicketForStaff)
	staff.Get("/:staffId", middleware.Protected(), validate.GetById("staffId"), handler.GetStaffById)
	staff.Post("/", middleware.Protected(), validate.CreateStaff(), handler.CreateStaff)
	staff.Put("/:staffId", middleware.Protected(), validate.EditStaff("staffId"), handler.EditStaff)
	staff.Delete("/", middleware.Protected(), validate.Delete(), handler.DeleteStaff)
	staff.Patch("/:staffId/active/:isActive", middleware.Protected(), validate.ActiveStaff(), handler.ActiveStaff)
	account.Post("/change-password", middleware.Protected(), validate.StaffChangePassword(), handler.StaffChangePassword)

	customer := v1.Group("/customer", logger.New())

	customer.Get("/", middleware.Protected(), handler.GetCustomer)
	customer.Get("/:customerId", middleware.Protected(), validate.GetById("customerId"), handler.GetCustomerById)

	chain := v1.Group("/chain", logger.New())
	chain.Get("/", middleware.Protected(), handler.GetCinemaChain)
	chain.Get("/:chainId", middleware.Protected(), handler.GetCinemaChainById)
	chain.Post("/", middleware.Protected(), validate.CreateCinemaChain(), handler.CreateCinemaChain)
	chain.Put("/:chainId", middleware.Protected(), validate.EditCinemaChain("chainId"), handler.EditCinemaChain)
	chain.Delete("/", middleware.Protected(), validate.Delete(), handler.DeleteCinamaChain)

	cinema := v1.Group("/cinema", logger.New())
	cinema.Get("/", middleware.Protected(), handler.GetCinema)
	cinema.Get("/:cinemaId", middleware.Protected(), handler.GetCinemaById)
	cinema.Post("/", middleware.Protected(), validate.CreateCinema(), handler.CreateCinema)
	cinema.Put("/:cinemaId", middleware.Protected(), validate.EditCinema("cinemaId"), handler.EditCinema)
	cinema.Delete("/", middleware.Protected(), validate.Delete(), handler.DeleteCinema)
	cinema.Get("/:cinemaId/rooms", middleware.Protected(), handler.GetRoomsByCinemaId)

	room := v1.Group("/room", logger.New())
	room.Get("/", middleware.Protected(), handler.GetRoom)
	room.Get("/:roomId", middleware.Protected(), handler.GetRoomById)
	room.Post("/", middleware.Protected(), validate.CreateRoom(), handler.CreateRoom)
	room.Put("/:roomId", middleware.Protected(), validate.EditRoom("roomId"), handler.EditRoom)
	room.Delete("/", middleware.Protected(), validate.Delete(), handler.DeleteRoom)
	room.Patch("/:roomId/disable", middleware.Protected(), validate.DisableRoom("roomId"), handler.DisableRoom)
	room.Patch("/:roomId/enable", middleware.Protected(), validate.EnableRoom("roomId"), handler.EnableRoom)

	formats := v1.Group("/formats", logger.New())
	formats.Get("/", middleware.Protected(), handler.GetFormats)

	actor := v1.Group("/actor", logger.New())
	actor.Post("/", middleware.Protected(), validate.CreateActor(), handler.CreateActor)
	actor.Post("/create-bulk", middleware.Protected(), validate.CreateActors, handler.CreateActors)
	actor.Put("/:actorId", middleware.Protected(), validate.UpdateActor("actorId"), handler.UpdateActor)
	actor.Get("/", middleware.Protected(), handler.GetActor)
	actor.Get("/movie/:movieId", middleware.Protected(), validate.GetById("movieId"), handler.GetActorsByMovieId)

	director := v1.Group("/director", logger.New())
	director.Get("/", middleware.Protected(), handler.GetDirector)
	director.Get("/:directorId", middleware.Protected(), handler.GetDirectorById)
	director.Get("/movie/:movieId", middleware.Protected(), validate.GetById("movieId"), handler.GetDirectorByMovieId)
	director.Post("/", middleware.Protected(), validate.CreateDirector(), handler.CreateDirector)
	director.Put("/:directorId", middleware.Protected(), validate.UpdateDirector("directorId"), handler.UpdateDirector)
	director.Delete("/", middleware.Protected(), validate.Delete(), handler.DeleteDirector)

	v1.Post("/cloudinary-signature", middleware.Protected(), handler.GenerateSignature)
	movie := v1.Group("/movie", logger.New())

	movie.Get("/", middleware.Protected(), handler.GetMovies)
	movie.Put("/disable", middleware.Protected(), validate.DisableMovie(), handler.DisableMovie)
	movie.Get("/:movieId", middleware.Protected(), handler.GetMovieById)
	movie.Post("/", middleware.Protected(), validate.CreateMovie(), handler.CreateMovie)
	movie.Put("/:movieId", middleware.Protected(), validate.EditMovie("movieId"), handler.EditMovie)
	movie.Patch("/approve/:movieId", middleware.Protected(), validate.ApproveMovie("movieId"), handler.ApproveMovie)

	//movie.Post("/:movieId/media", middleware.Protected(), validate.UploadMovieMedia("movieId"), handler.UploadMovieMedia)

	movie.Post("/:movieId/poster", middleware.Protected(), validate.UploadMoviePoster("movieId"), handler.UploadMultiplePosters)
	movie.Post("/:movieId/trailer", middleware.Protected(), validate.UploadMovieTrailer("movieId"), handler.UploadMultipleTrailers)

	schedule := v1.Group("/schedule", logger.New())
	schedule.Get("/", middleware.Protected(), handler.GetScheduleTemplate)
	schedule.Get("/:scheduleTemplateId", middleware.Protected(), validate.ValidScheduleTemplateId, handler.GetScheduleTemplateById)
	schedule.Post("/", middleware.Protected(), validate.CreateScheduleTemplate(), handler.CreateScheduleTemplate)
	schedule.Put("/:scheduleTemplateId", middleware.Protected(), validate.UpdateSchedulerTemplate("scheduleTemplateId"), handler.UpdateSchedulerTemplate)
	schedule.Delete("/:scheduleTemplateId", middleware.Protected(), validate.ValidScheduleTemplateId, handler.DeleteScheduleTemplate)

	holidays := v1.Group("/holidays", logger.New())
	holidays.Get("/", middleware.Protected(), handler.GetHoliday)
	holidays.Post("/", middleware.Protected(), validate.CreateHoliday(), handler.CreateHoliday)
	holidays.Put("/:holidayId", middleware.Protected(), validate.UpdateHoliday("holidayId"), handler.UpdateHoliday)
	holidays.Delete("/:id", middleware.Protected(), handler.DeleteHoliday)

	showtime := v1.Group("/showtime", logger.New())
	showtime.Get("/", middleware.Protected(), handler.GetShowtime)
	showtime.Post("/create-ticket", middleware.Protected(), validate.CreateTicket(), handler.CreateTicket)
	showtime.Get("/staff", middleware.Protected(), handler.GetShowtimeForStaff)
	showtime.Get("/:cinemaId", middleware.Protected(), handler.GetShowtimeByCinemaIdAndDate)
	showtime.Get("/:showtimeId", middleware.Protected(), validate.GetShowtimeById("showtimeId"), handler.GetShowtimeById)
	showtime.Get("/:id/tickets", middleware.Protected(), handler.GetShowtimeTicket)
	showtime.Get("/:id/seats", middleware.Protected(), handler.GetShowtimeSeat)
	showtime.Post("/", middleware.Protected(), validate.CreateShowtimeBatch(), handler.CreateShowtimeBatch)
	showtime.Post("/auto-generate", middleware.Protected(), validate.AutoGenerateShowtimeSchedule(), handler.AutoGenerateShowtimeSchedule)
	showtime.Put("/:showtimeId", middleware.Protected(), validate.EditShowtime("showtimeId"), handler.EditShowtime)
	showtime.Delete("/:showtimeId", middleware.Protected(), validate.DeleteShowtime("showtimeId"), handler.DeleteShowtime)

	ticketseller := v1.Group("/ticket", logger.New())

	ticketseller.Get("/", middleware.Protected(), handler.GetTicket)

	statistic := v1.Group("/statistic", logger.New())
	statistic.Get("/", middleware.Protected(), handler.GetAdminStats)
	// Admin
	ticketseller.Get("/admin", middleware.Protected(), handler.GetTicketAdmin)

	// Public

	// ROUTES
	app.Post("/payments", handler.CreatePayment)
	app.Get("/vnpay/return", handler.VNPayCallback) // Callback từ VNPay
	app.Post("/vnpay/ipn", handler.VNPayIPN)
	// Server-to-Server
	chuoirap := v1.Group("/chuoi-rap")
	chuoirap.Get("/tinh", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetCinemaChainsByArea)
	chuoirap.Get("/khu-vuc", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetProvincesWithChains)
	//người dùng
	rap := v1.Group("/rap")
	rap.Get("/", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetCinemas)
	rap.Get("/search", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.SearchCinemas)
	rap.Get("/tinh", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetCinemasByProvince)
	rap.Get("/dia-chi", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetCinemaProvinces)
	rap.Get("/:slug", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetCinemaDetail)
	rap.Get("/:slug/lich-chieu", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetShowtimeByCinemaId)
	phim := v1.Group("/phim")
	phim.Get("/search", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.SearchMovies)
	phim.Get("/dang-chieu", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetMovieNowShowing)
	phim.Get("/sap-chieu", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetMovieUpcoming)
	phim.Get("/status/:status", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetMoviesByStatus)
	phim.Get("/genres", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetMovieGenres)
	phim.Get("/:slug", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetMovieDetail)
	phim.Get("/:movieId/lich-chieu", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetShowtimesByMovie)

	lichchieu := v1.Group("/lich-chieu")
	lichchieu.Get("/", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetShowtimes)
	lichchieu.Get("/dat-ve/:code", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetShowtimeByPublicCode)
	lichchieu.Get("/phim", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetShowtimesByMovieAndProvince)
	lichchieu.Get("/:code/ghe", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetSeatsByShowtime)
	lichchieu.Post("/:code/giu-ghe", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.HoldSeat)
	lichchieu.Post("/:code/tra-ghe", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.ReleaseSeat)
	lichchieu.Get("/:code/ghe-giu", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetHeldSeatsBySession)
	lichchieu.Post("/:code/thanh-toan", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.PurchaseSeats)

	lichchieu.Get("/ghe/:showtimeId", middleware.OptionalJWT(), middleware.OptionalAuth(), websocket.New(handler.SeatWebsocket))

	donhang := v1.Group("/don-hang")
	donhang.Get("/", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetMyOrders)
	donhang.Get("/:orderCode", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetOrderDetail)
	donhang.Post("/cancel-by-code", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.CancelOrderByCode)
	donhang.Post("/:publicCode/cancel", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.CancelOrderByUser)
	khachhang := v1.Group("/khach-hang")
	khachhang.Post("/refresh-token", handler.RefreshCustomerToken)
	khachhang.Post("/login", handler.CustomerLogin)
	khachhang.Get("/me", middleware.OptionalJWT(), middleware.OptionalAuth(), handler.GetCurrentCustomer)
	khachhang.Post("/register", validate.RegisterCustomer(), handler.RegisterCustomer)
	khachhang.Post("/change-password", middleware.OptionalJWT(), middleware.OptionalAuth(), validate.ChangePasswordCustomer(), handler.ChangePasswordCustomer)
	khachhang.Post("/forgot-password", middleware.OptionalJWT(), middleware.OptionalAuth(), validate.ForgetPassword(), handler.ForgotPassword)
	khachhang.Post("/reset-password", middleware.OptionalJWT(), middleware.OptionalAuth(), validate.RestPassword(), handler.ResetPassword)
}
