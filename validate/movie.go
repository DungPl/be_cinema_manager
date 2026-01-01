package validate

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func CreateMovie() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateMovieInput
		// Parse JSON từ request body vào struct
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		// Validate input
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền"))
		}

		// // Kiểm tra Director tồn tại
		var director model.Director
		if input.DirectorId != nil {
			if err := database.DB.First(&director, input.DirectorId).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Đạo diễn không tồn tại", fmt.Errorf("directorId not found"), "directorId")
				}
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
				})
			}
		}
		// // Kiểm tra ActorIds tồn tại
		var actors []model.Actor
		if len(input.ActorIds) > 0 {
			if err := database.DB.Where("id IN ? ", input.ActorIds).Find(&actors).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
				})
			}
			if len(actors) != len(input.ActorIds) {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Một hoặc nhiều diễn viên không tồn tại", fmt.Errorf("some actorIds not found"), "actorIds")
			}
		}
		// Kiểm tra định dạng tồn tại
		var formats []model.Format
		if err := database.DB.Where("id IN ?", input.FormatIds).Find(&formats).Error; err != nil { /* ... */
		}
		if len(formats) != len(input.FormatIds) {
			return utils.ErrorResponse(c, 400, "Định dạng không tồn tại", nil)
		}
		// Kiểm tra phim đã tồn tại (dựa trên title)
		var existingMovie model.Movie
		if err := database.DB.Where("title = ? AND deleted_at IS NULL", input.Title).First(&existingMovie).Error; err == nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phim đã tồn tại", fmt.Errorf("movie title already exists"), "title")
		}

		// Kiểm tra ngày hợp lệ
		if input.DateSoon != nil && input.DateSoon.Time.After(input.DateRelease.Time) {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Ngày chiếu sớm không được sau ngày khởi chiếu", fmt.Errorf("invalid dateSoon"), "dateSoon")
		}
		if input.DateEnd != nil && input.DateEnd.Before(input.DateRelease.Time) {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Ngày kết thúc không được trước ngày khởi chiếu", fmt.Errorf("invalid dateEnd"), "dateEnd")
		}

		c.Locals("inputCreateMovie", input)
		c.Locals("formats", formats)
		return c.Next()
	}
}
func EditMovie(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		var input model.EditMovieInput
		// Parse JSON từ request body vào struct
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		// Validate input
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
		}
		// Kiểm tra Movie tồn tại
		var movie model.Movie
		if err := database.DB.First(&movie, valueKey).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Không tìm thấy phim",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}

		// Kiểm tra title không trùng (trừ bản ghi hiện tại)
		var existingMovie model.Movie
		if err := database.DB.Where("title = ? AND id != ? ", movie.Title, valueKey).First(&existingMovie).Error; err == nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên phim đã tồn tại", fmt.Errorf("movie title already exists"), "title")
		}

		// Kiểm tra ngày hợp lệ
		if input.DateSoon != nil && input.DateSoon.After(input.DateRelease.Time) {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Ngày chiếu sớm không được sau ngày khởi chiếu", fmt.Errorf("invalid dateSoon"), "dateSoon")
		}
		if input.DateRelease != nil {
			if input.DateEnd != nil && input.DateEnd.Before(input.DateRelease.Time) {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Ngày kết thúc không được trước ngày khởi chiếu", fmt.Errorf("invalid dateEnd"), "dateEnd")
			}
		}
		if input.DateEnd != nil && input.DateEnd.Before(movie.DateRelease.Time) {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Ngày kết thúc không được trước ngày khởi chiếu", fmt.Errorf("invalid dateEnd"), "dateEnd")
		}
		var formatIds []uint
		if input.FormatIds != nil && len(*input.FormatIds) > 0 {
			formatIds = *input.FormatIds // deref into local slice (non-pointer)
			var count int64
			if err := database.DB.Model(&model.Format{}).Where("id IN ?", formatIds).Count(&count).Error; err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn định dạng", err)
			}
			if count != int64(len(formatIds)) {
				return utils.ErrorResponse(c, fiber.StatusBadRequest, "Một số định dạng không tồn tại", nil)
			}
		}
		c.Locals("inputEditMovie", input)
		c.Locals("movieId", uint(valueKey))
		c.Locals("formatIds", formatIds)
		return c.Next()
	}
}
func ApproveMovie(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
		}
		// Kiểm tra Movie tồn tại
		var movie model.Movie
		if err := database.DB.Where(" is_available IS false").First(&movie, valueKey).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Không tìm thấy phim",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}
		c.Locals("movieId", uint(valueKey))
		return c.Next()
	}
}
func DisableMovie() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.ArrayId
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		// Validate input
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		if len(input.IDs) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Mảng ID cần xoá không được để trống"})
		}
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
		}
		// Kiểm tra Movie tồn tại
		var movies []model.Movie
		if err := database.DB.Where("id IN ? AND is_available = true", input.IDs).Find(&movies).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}

		// Nếu không tìm thấy phim nào
		if len(movies) == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Không tìm thấy phim hoặc phim đã bị vô hiệu hóa",
			})
		}
		// Kiểm tra xem phim có liên quan đến lịch chiếu không
		// Lấy thời gian hiện tại (múi giờ Việt Nam)
		current := time.Now().In(time.FixedZone("ICT", 7*3600))

		// Kiểm tra xem có suất chiếu nào còn hiệu lực (start_time >= hôm nay)
		var activeShowtimeCount int64
		if err := database.DB.Model(&model.Showtime{}).
			Where("movie_id IN ?", input.IDs).
			Where("start_time >= ?", current).
			Count(&activeShowtimeCount).Error; err != nil {

			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi kiểm tra lịch chiếu: %s", err.Error()),
			})
		}

		// Nếu có suất chiếu tương lai → không cho xóa
		if activeShowtimeCount > 0 {
			return utils.ErrorResponseHaveKey(
				c,
				fiber.StatusBadRequest,
				"Không thể xóa phim vì có suất chiếu chưa diễn ra",
				fmt.Errorf("upcoming showtimes exist"),
				"ids",
			)
		}
		c.Locals("ids", input.IDs)
		return c.Next()
	}
}
