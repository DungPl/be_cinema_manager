package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func UpdateMovieStatusJob() {
	db := database.DB
	now := time.Now()

	var movies []model.Movie
	db.Find(&movies)

	for _, movie := range movies {
		var count int64
		db.Model(&model.Showtime{}).
			Where("movie_id = ? AND start_time > ? AND status = ?", movie.ID, now, "ACTIVE").
			Count(&count)

		var newStatus string

		if movie.DateRelease.After(now) {
			newStatus = "COMING_SOON"
		} else if count > 0 {
			newStatus = "NOW_SHOWING"
		} else {
			newStatus = "ENDED"
		}

		if movie.StatusMovie != newStatus {
			db.Model(&model.Movie{}).
				Where("id = ?", movie.ID).
				Update("status_movie", newStatus)
		}
	}
}

func GetMovies(c *fiber.Ctx) error {
	filterInput := new(model.FilterMoviInput)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}
	db := database.DB
	_, isAdmin, isQuanLy, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isKiemDuyet && !isQuanLy {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
	}
	condition := db.Model(&model.Movie{})
	// Áp dụng bộ lọc
	// Thời gian hiện tại (theo múi giờ +07:00)
	currentTime := time.Now().In(time.FixedZone("ICT", 7*3600))

	// Áp dụng bộ lọc thời gian thực dựa trên ShowingStatus
	if filterInput.ShowingStatus != "" {
		switch filterInput.ShowingStatus {
		case "COMING_SOON":
			// Ưu tiên phim admin set COMING_SOON
			// Nếu không có thì fallback phim chưa công chiếu (date_release > now)
			condition = condition.Where(
				"(status_movie = 'COMING_SOON') OR "+
					"(status_movie IS NULL AND date_release > ? AND (date_end IS NULL OR date_end > ?))",
				currentTime, currentTime,
			)

		case "NOW_SHOWING":
			// Ưu tiên phim admin set NOW_SHOWING
			// Fallback phim đang chiếu theo ngày
			condition = condition.Where(
				"(status_movie = 'NOW_SHOWING') OR "+
					"(status_movie IS NULL AND date_release <= ? AND (date_end IS NULL OR date_end >= ?))",
				currentTime, currentTime,
			)

		case "ENDED":
			// Ưu tiên phim admin set ENDED
			// Fallback phim đã kết thúc theo ngày
			condition = condition.Where(
				"(status_movie = 'ENDED') OR "+
					"(status_movie IS NULL AND date_end IS NOT NULL AND date_end < ?)",
				currentTime,
			)
		}
	}
	if filterInput.Country != "" {
		condition = condition.Where("LOWER(country) LIKE ?", "%"+strings.ToLower(filterInput.Country)+"%")
	}
	if filterInput.Genre != "" {
		condition = condition.Where("LOWER(genre) LIKE ?", "%"+strings.TrimSpace(filterInput.Genre)+"%")
	}
	if filterInput.Title != "" {
		condition = condition.Where("title LIKE ?", "%"+strings.ToLower(filterInput.Title)+"%")
	}
	if filterInput.Duration > 0 {
		condition = condition.Where("duration = ?", filterInput.Duration)
	}
	if filterInput.Language != "" {
		condition = condition.Where("language LIKE ?", "%"+strings.ToLower(filterInput.Language)+"%")
	}
	if filterInput.DateRelease != nil {
		startOfDay := filterInput.DateRelease.Truncate(24 * time.Hour)
		endOfDay := startOfDay.Add(24*time.Hour - time.Second)
		condition = condition.Where("date_release BETWEEN ? AND ?", startOfDay, endOfDay)
	}
	var totalCount int64
	condition.Count(&totalCount)
	var movies []model.Movie
	condition = utils.ApplyPagination(condition, filterInput.Limit, filterInput.Page)
	condition.Preload("Formats").
		Preload("Director").
		Preload("Actors").
		Preload("Posters").
		Preload("Trailers").
		Preload("AccountModerator").
		Order("id DESC, date_release DESC").Find(&movies)
	response := &model.ResponseCustom{
		Rows:       movies,
		Limit:      filterInput.Limit,
		Page:       filterInput.Page,
		TotalCount: totalCount,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetMovieById(c *fiber.Ctx) error {
	idParam := c.Params("movieId") // string
	movieId64, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}
	movieId := uint(movieId64)
	db := database.DB
	var movie model.Movie
	db.Preload("Director").Preload("Actors").Preload("Posters").Preload("Trailers").Preload("AccountModerator").First(&movie, movieId)
	return utils.SuccessResponse(c, fiber.StatusOK, movie)
}
func CreateMovie(c *fiber.Ctx) error {
	db := database.DB
	movieInput, ok := c.Locals("inputCreateMovie").(model.CreateMovieInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	formats := c.Locals("formats").([]model.Format)
	tx := db.Begin()
	// Xử lý Director
	// --- XỬ LÝ DIRECTOR ---
	var directorId uint
	if movieInput.DirectorId != nil {
		var director model.Director
		if err := tx.First(&director, movieInput.DirectorId).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Đạo diễn không tồn tại", nil, "directorId")
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi DB", err)
		}
		directorId = director.ID
	} else if movieInput.DirectorName != nil {
		name := strings.TrimSpace(*movieInput.DirectorName)
		if name == "" {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên đạo diễn không được rỗng", nil, "directorName")
		}

		var director model.Director
		if err := tx.Where("LOWER(name) = LOWER(?)", name).First(&director).Error; err == nil {
			directorId = director.ID
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			director = model.Director{Name: name}
			if err := tx.Create(&director).Error; err != nil {
				tx.Rollback()
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tạo đạo diễn", err)
			}
			directorId = director.ID
		} else {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn đạo diễn", err)
		}
	} else {
		tx.Rollback()
		return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phải cung cấp directorId hoặc directorName", nil, "directorId")
	}

	// --- XỬ LÝ ACTORS ---
	var actorIds []uint
	seen := make(map[uint]bool) // tránh trùng

	// 1. Từ ActorIds
	if len(movieInput.ActorIds) > 0 {
		var actors []model.Actor
		if err := tx.Where("id IN ?", movieInput.ActorIds).Find(&actors).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi kiểm tra diễn viên", err)
		}
		if len(actors) != len(movieInput.ActorIds) {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Một số diễn viên không tồn tại", nil, "actorIds")
		}
		for _, a := range actors {
			if !seen[a.ID] {
				actorIds = append(actorIds, a.ID)
				seen[a.ID] = true
			}
		}
	}

	// 2. Từ ActorNames
	for _, name := range movieInput.ActorNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		var actor model.Actor
		if err := tx.Where("LOWER(name) = LOWER(?)", name).First(&actor).Error; err == nil {
			if !seen[actor.ID] {
				actorIds = append(actorIds, actor.ID)
				seen[actor.ID] = true
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			actor = model.Actor{Name: name}
			if err := tx.Create(&actor).Error; err != nil {
				tx.Rollback()
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tạo diễn viên", err)
			}
			actorIds = append(actorIds, actor.ID)
			seen[actor.ID] = true
		} else {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn diễn viên", err)
		}
	}
	dataInfo, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
	accountId := dataInfo.AccountId

	movie := model.Movie{
		Genre:              movieInput.Genre,
		Title:              movieInput.Title,
		Description:        movieInput.Description,
		Duration:           movieInput.Duration,
		Country:            movieInput.Country,
		DateRelease:        movieInput.DateRelease,
		Language:           movieInput.Language,
		DateSoon:           movieInput.DateSoon,
		DateEnd:            movieInput.DateEnd,
		DirectorId:         directorId,
		AgeRestriction:     movieInput.AgeRestriction,
		AccountModeratorId: accountId,
		Slug:               helper.GenerateUniquerateUniqueMoviSlug(tx, movieInput.Title),
	}
	// ✅ Xác định trạng thái phim
	now := time.Now()

	if movieInput.DateRelease.IsZero() {
		movie.StatusMovie = "UNKNOWN"
	} else if movieInput.DateRelease.Time.After(now) {
		movie.StatusMovie = "COMING_SOON"
	} else if movieInput.DateEnd != nil && movieInput.DateEnd.Before(now) {
		movie.StatusMovie = "ENDED"
	} else {
		movie.StatusMovie = "NOW_SHOWING"
	}
	if isAdmin {
		movie.IsAvailable = true
	} else if isKiemDuyet {
		movie.IsAvailable = false
	}
	if err := tx.Create(&movie).Error; err != nil {
		return err
	}
	for _, f := range formats {
		if err := tx.Create(&model.MovieFormat{
			MovieId:  movie.ID,
			FormatId: f.ID,
		}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi lưu format phim", err)
		}
	}
	// Tạo MovieActor entries
	// --- TẠO MovieActor ---
	for _, actorId := range actorIds {
		if err := tx.Create(&model.MovieActor{
			MovieId: movie.ID,
			ActorId: actorId,
		}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi liên kết diễn viên", err)
		}
	}

	// --- COMMIT ---
	if err := tx.Commit().Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi commit", err)
	}

	// --- LOAD LẠI VỚI RELATION ---
	var createdMovie model.Movie
	if err := database.DB.
		Preload("Director").
		Preload("Actors").
		Preload("AccountModerator").
		First(&createdMovie, movie.ID).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tải phim", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, movie)
}
func EditMovie(c *fiber.Ctx) error {
	db := database.DB
	movieId := c.Locals("movieId").(uint)
	movieInput, ok := c.Locals("inputEditMovie").(model.EditMovieInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	formatIds := c.Locals("formatIds").([]uint)
	tx := db.Begin()
	var movie model.Movie
	tx.Preload("Director").Preload("Actors").Preload("Posters").Preload("Trailers").Preload("AccountModerator").First(&movie, movieId)
	var directorId uint
	if movieInput.DirectorId != nil {
		var director model.Director
		if err := tx.Where("id = ? ", *movieInput.DirectorId).First(&director).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("đạo diễn không tồn tại")
			}
			return err
		}
		directorId = director.ID
	} else if movieInput.DirectorName != nil {
		var director model.Director
		if err := tx.Where("name = ? ", *movieInput.DirectorName).First(&director).Error; err == nil {
			directorId = director.ID
		} else if err == gorm.ErrRecordNotFound {
			director = model.Director{Name: *movieInput.DirectorName}
			if err := tx.Create(&director).Error; err != nil {
				return err
			}
			directorId = director.ID
		} else {
			return err
		}
	}
	var actorIds []uint
	if movieInput.ActorIds != nil && len(*movieInput.ActorIds) > 0 {
		var actors []model.Actor
		if err := tx.Where("id IN ?", *movieInput.ActorIds).Find(&actors).Error; err != nil {
			return err
		}
		if len(actors) != len(*movieInput.ActorIds) {
			return fmt.Errorf("một hoặc nhiều diễn viên không tồn tại")
		}
		for _, actor := range actors {
			actorIds = append(actorIds, actor.ID)
		}
	}
	if movieInput.ActorNames != nil && len(*movieInput.ActorNames) > 0 {
		for _, name := range *movieInput.ActorNames {
			var actor model.Actor
			if err := tx.Where("name = ?", name).First(&actor).Error; err == nil {
				actorIds = append(actorIds, actor.ID)
			} else if err == gorm.ErrRecordNotFound {
				actor = model.Actor{Name: name}
				if err := tx.Create(&actor).Error; err != nil {
					return err
				}
				actorIds = append(actorIds, actor.ID)
			} else {
				return err
			}
		}
	}
	if len(formatIds) > 0 {
		// Xóa cũ
		if err := tx.Where("movie_id = ?", movie.ID).Delete(&model.MovieFormat{}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Lỗi xóa định dạng", err)
		}

		// Thêm mới
		for _, fid := range formatIds {
			if err := tx.Create(&model.MovieFormat{MovieId: movie.ID, FormatId: fid}).Error; err != nil {
				tx.Rollback()
				return utils.ErrorResponse(c, 500, "Lỗi thêm định dạng", err)
			}
		}
	}
	if movieInput.Genre != nil {
		movie.Genre = *movieInput.Genre
	}
	if movieInput.Title != nil {
		movie.Title = *movieInput.Title
	}
	if movieInput.Duration != nil {
		movie.Duration = *movieInput.Duration
	}
	if movieInput.Language != nil {
		movie.Language = *movieInput.Language
	}
	if movieInput.Description != nil {
		movie.Description = *movieInput.Description
	}
	if movieInput.AgeRestriction != nil {
		movie.AgeRestriction = *movieInput.AgeRestriction
	}
	if movieInput.StatusMovie != nil {
		movie.StatusMovie = *movieInput.StatusMovie
	}
	if movieInput.Country != nil {
		movie.Country = *movieInput.Country
	}
	if movieInput.DateSoon != nil {
		movie.DateSoon = movieInput.DateSoon
	}
	if movieInput.DateRelease != nil {
		movie.DateRelease = *movieInput.DateRelease
	}
	if movieInput.DateEnd != nil {
		movie.DateEnd = movieInput.DateEnd
	}
	movie.DirectorId = directorId
	movie.Slug = helper.GenerateUniquerateUniqueMoviSlug(tx, movie.Title)
	if err := tx.Model(&model.Movie{}).Where("id = ?", movieId).Updates(movie).Error; err != nil {
		return err
	}

	// Xóa MovieActor hiện tại
	if err := tx.Where("movie_id = ?", movieId).Delete(&model.MovieActor{}).Error; err != nil {
		return err
	}

	// Tạo lại MovieActor entries
	for _, actorId := range actorIds {
		movieActor := model.MovieActor{
			MovieId: uint(movieId),
			ActorId: actorId,
		}
		if err := tx.Create(&movieActor).Error; err != nil {
			return err
		}
	}
	var updatedMovie model.Movie
	if err := tx.
		Preload("Director").
		Preload("Actors").
		Preload("Posters").
		Preload("Trailers").
		Preload("AccountModerator").
		First(&updatedMovie, movieId).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể lấy thông tin phim: %s", err.Error()),
		})
	}
	tx.Commit()
	return utils.SuccessResponse(c, fiber.StatusOK, updatedMovie)
}
func ApproveMovie(c *fiber.Ctx) error {
	db := database.DB
	movieId := c.Locals("movieId").(uint)
	tx := db.Begin()
	var movie model.Movie
	tx.Preload("Director").Preload("Actors").Preload("Posters").Preload("Trailers").Preload("AccountModerator").First(&movie, movieId)
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
	}
	movie.IsAvailable = true
	db.Save(&movie).Scan(&movie)
	return utils.SuccessResponse(c, fiber.StatusOK, movie)
}
func DisableMovie(c *fiber.Ctx) error {
	ids, ok := c.Locals("ids").([]uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy danh sách ID",
		})
	}
	db := database.DB
	tx := db.Begin()

	// Soft delete Movie
	if err := tx.Model(&model.Movie{}).Where("id IN ?", ids).Update("is_available", false).Error; err != nil {
		fmt.Println("Error:", err)
	}
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Commit transaction thất bại",
		})
	}
	return utils.SuccessResponse(c, fiber.StatusOK, ids)
}
func SearchMovies(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return utils.ErrorResponse(c, 400, "Thiếu từ khóa tìm kiếm", nil)
	}

	var movies []model.Movie
	err := database.DB.
		Where("title ILIKE ?  ", "%"+query+"%").
		Preload("Posters").
		Preload("Trailers").
		Limit(20).
		Find(&movies).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi tìm kiếm phim", err)
	}

	return utils.SuccessResponse(c, 200, movies)
}
func GetMovieNowShowing(c *fiber.Ctx) error {
	var movies []model.Movie

	err := database.DB.
		Where("status_movie = ?", "NOW_SHOWING").
		Preload("Posters").
		Preload("Trailers").
		Preload("Director").
		Preload("Formats").
		Preload("Actors").
		Find(&movies).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy phim đang chiếu", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, movies)

}
func GetMovieUpcoming(c *fiber.Ctx) error {
	var movies []model.Movie

	err := database.DB.
		Where("status_movie = ?", "UP_COMING").
		Preload("Posters").
		Preload("Trailers").
		Preload("Formats").
		Preload("Director").
		Preload("Actors").
		Find(&movies).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy phim đang chiếu", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, movies)
}
func GetMovieDetail(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return utils.ErrorResponse(c, 400, "slug is required", nil)
	}

	var movie model.Movie
	err := database.DB.
		Preload("Posters").
		Preload("Trailers").
		Preload("Director").
		Preload("Actors").
		Preload("Formats").
		Where("slug = ?", slug).
		First(&movie).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, 404, "Không tìm thấy phim", err)
		}
		return utils.ErrorResponse(c, 500, "Lỗi tải chi tiết phim", err)
	}

	return utils.SuccessResponse(c, 200, movie)
}
func GetShowtimesByMovie(c *fiber.Ctx) error {
	movieId := c.Params("movieId")
	date := c.Query("date")

	var showtimes []model.Showtime
	db := database.DB
	query := db.Where("movie_id = ?", movieId)

	// Nếu client truyền ngày → lọc theo ngày YYYY-MM-DD
	if date != "" {
		query = query.Where("DATE(start_time) = ?", date)
	} else {
		// Nếu không truyền → lấy các suất chiếu trong tương lai
		query = query.Where("start_time >= NOW()")
	}

	err := query.
		Preload("Room").
		Preload("Movie").
		Preload("Room.Cinema").
		Find(&showtimes).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Không lấy được suất chiếu", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, showtimes)
}

func GetMoviesByStatus(c *fiber.Ctx) error {
	status := c.Params("status")
	if status == "" {
		return utils.ErrorResponse(c, 400, "Thiếu status phim", nil)
	}

	validStatuses := map[string]bool{
		"NOW_SHOWING": true,
		"COMING_SOON": true,
		"EARLY_SHOW":  true,
	}
	if !validStatuses[status] {
		return utils.ErrorResponse(c, 400, "Status phim không hợp lệ", nil)
	}

	var movies []model.Movie
	query := database.DB.
		Preload("Posters").
		Preload("Trailers").
		Where("status_movie = ?", status)

	// Phim chiếu sớm: DateSoon không null
	if status == "EARLY_SHOW" {
		query = query.Where("date_soon IS NOT NULL")
	}

	if err := query.Find(&movies).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy danh sách phim", err)
	}

	return utils.SuccessResponse(c, 200, movies)
}
func GetMovieGenres(c *fiber.Ctx) error {
	var genres []string
	err := database.DB.
		Model(&model.Movie{}).
		Distinct("genre").
		Pluck("genre", &genres).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy danh sách thể loại", err)
	}

	// Lọc bỏ rỗng
	uniqueGenres := make([]string, 0)
	seen := make(map[string]bool)
	for _, g := range genres {
		if g != "" && !seen[g] {
			seen[g] = true
			uniqueGenres = append(uniqueGenres, g)
		}
	}

	return utils.SuccessResponse(c, 200, uniqueGenres)
}
