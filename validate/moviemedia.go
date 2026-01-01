package validate

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func UploadMovieMedia(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		_, isAdmin, isQuanLy, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isQuanLy && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("Bạn không có thẩm quyền "))
		}
		// Kiểm tra Movie tồn tại
		var movie model.Movie
		if err := database.DB.Where("deleted_at IS NULL").First(&movie, valueKey).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return utils.ErrorResponseHaveKey(c, fiber.StatusNotFound, "Phim không tồn tại", fmt.Errorf("movie not found"), "movieId")
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}
		// Kiểm tra file poster
		posterFile, err := c.FormFile("poster")
		if err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "File poster không được cung cấp", fmt.Errorf("missing poster file"), "poster")
		}

		// Kiểm tra định dạng poster (chỉ JPG, PNG, JPEG)
		extPoster := filepath.Ext(posterFile.Filename)
		if extPoster != ".jpg" && extPoster != ".jpeg" && extPoster != ".png" {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Định dạng poster không hỗ trợ (chỉ JPG, PNG, JPEG)", fmt.Errorf("invalid poster format"), "poster")
		}

		// Kiểm tra kích thước poster (tối đa 5MB)
		if posterFile.Size > 5*1024*1024 {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "File poster quá lớn (tối đa 5MB)", fmt.Errorf("poster too large"), "poster")
		}

		// Kiểm tra file trailer
		trailerFile, err := c.FormFile("trailer")
		if err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "File trailer không được cung cấp", fmt.Errorf("missing trailer file"), "trailer")
		}

		// Lấy isPrimary từ form data
		isPrimary, err := strconv.ParseBool(c.FormValue("isPrimary"))
		if err != nil {
			isPrimary = false
		}
		// Khởi tạo Cloudinary
		cld, err := cloudinary.NewFromParams(
			os.Getenv("CLOUDINARY_CLOUD_NAME"),
			os.Getenv("CLOUDINARY_API_KEY"),
			os.Getenv("CLOUDINARY_API_SECRET"),
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Không thể khởi tạo Cloudinary: %s", err.Error()),
			})
		}
		c.Locals("movieId", uint(valueKey))
		c.Locals("posterFile", posterFile)
		c.Locals("trailerFile", trailerFile)
		c.Locals("isPrimary", isPrimary)
		c.Locals("cld", cld)
		return c.Next()
	}
}

// handlers/movie_media_handler.go
func UploadMoviePoster(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "ID không hợp lệ", err)
		}

		// Quyền
		_, isAdmin, isQuanLy, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isQuanLy && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
		}

		// Tìm phim
		var movie model.Movie
		if err := database.DB.First(&movie, valueKey).Error; err != nil {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Phim không tồn tại", nil)
		}

		// Đọc form data
		form, _ := c.MultipartForm()
		var files []*multipart.FileHeader
		if form != nil {
			files = form.File["posters"]
		}

		// Không bắt buộc có files (khác với trước)
		c.Locals("movieId", uint(valueKey))
		c.Locals("posterFiles", files)

		var removeIds []string
		if form != nil {
			removeIds = form.Value["removePosterIds"]
		}

		c.Locals("removePosterIds", removeIds)
		c.Locals("removePosterIds", removeIds)

		// Xác định có muốn set primary hay không
		primaryPosterId := c.FormValue("primaryPosterId") // Ví dụ: "123" (ID cũ) hoặc "new_first" (mới đầu tiên)
		c.Locals("primaryPosterId", primaryPosterId)

		// Cloudinary
		cld, err := cloudinary.NewFromParams(
			os.Getenv("CLOUDINARY_CLOUD_NAME"),
			os.Getenv("CLOUDINARY_API_KEY"),
			os.Getenv("CLOUDINARY_API_SECRET"),
		)
		if err != nil {
			return utils.ErrorResponse(c, 500, "Cloudinary lỗi", err)
		}
		c.Locals("cld", cld)

		return c.Next()
	}
}

func UploadMovieTrailer(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "ID phim không hợp lệ", err)
		}

		// Quyền
		_, isAdmin, isQuanLy, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isQuanLy && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
		}

		// Check movie
		var movie model.Movie
		if err := database.DB.First(&movie, valueKey).Error; err != nil {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Phim không tồn tại", nil)
		}

		// Không xử lý file tại đây nữa!!!
		// Backend chỉ nhận JSON từ client Cloudinary upload

		c.Locals("movieId", uint(valueKey))

		cld, err := cloudinary.NewFromParams(
			os.Getenv("CLOUDINARY_CLOUD_NAME"),
			os.Getenv("CLOUDINARY_API_KEY"),
			os.Getenv("CLOUDINARY_API_SECRET"),
		)
		if err != nil {
			return utils.ErrorResponse(c, 500, "Không thể khởi tạo Cloudinary", err)
		}

		c.Locals("cld", cld)
		return c.Next()
	}
}
