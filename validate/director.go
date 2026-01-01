package validate

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// handlers/director_handler.go
func CreateDirector() fiber.Handler {
	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Không thể đọc form data", err)
		}
		name := form.Value["name"][0]
		nationality := form.Value["nationality"][0]
		var biography *string
		if vals, ok := form.Value["biography"]; ok && len(vals) > 0 && vals[0] != "" {
			bio := vals[0]   // tạo biến tạm
			biography = &bio // trỏ vào bio
		} else {
			biography = nil // không nhập -> nil
		}
		if name == "" {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên chuỗi rạp không được để trống", nil, "name")
		}
		// Kiểm tra quyền: chỉ admin hoặc kiểm duyệt
		_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền tạo đạo diễn", errors.New("permission denied"))
		}

		// Kiểm tra tên trùng (không phân biệt hoa thường)
		var existing model.Director
		if err := database.DB.Where("LOWER(name) = LOWER(?)", name).First(&existing).Error; err == nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên đạo diễn đã tồn tại", errors.New("name already exists"), "name")
		}
		input := model.CreateDirectorInput{
			Name:        name,
			Nationality: nationality,
			Biography:   biography,
		}
		// Khởi tạo Cloudinary
		cld, err := cloudinary.NewFromParams(
			os.Getenv("CLOUDINARY_CLOUD_NAME"),
			os.Getenv("CLOUDINARY_API_KEY"),
			os.Getenv("CLOUDINARY_API_SECRET"))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Không thể khởi tạo Cloudinary: %s", err.Error()),
			})
		}
		var avatarUrl string
		if files := form.File["avatar"]; len(files) > 0 {
			file := files[0]
			// Kiểm tra định dạng file
			ext := filepath.Ext(file.Filename)
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Định dạng file không hỗ trợ (chỉ hỗ trợ PNG, JPG, JPEG)", fmt.Errorf("invalid file format"), "logo")
			}
			// Mở file
			fileReader, err := file.Open()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Không thể đọc file avatar: %s", err.Error()),
				})
			}
			defer fileReader.Close()

			// Tải lên Cloudinary
			uploadResult, err := cld.Upload.Upload(context.Background(), fileReader, uploader.UploadParams{
				Folder:       "avatar_director",
				PublicID:     fmt.Sprintf("avatar_%s_%d", input.Name, time.Now().Unix()),
				ResourceType: "image",
			})
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Không thể tải lên Cloudinary: %s", err.Error()),
				})
			}

			avatarUrl = uploadResult.SecureURL
		} else if input.Avatar != nil && *input.Avatar != "" {
			avatarUrl = *input.Avatar
		}
		// Save input to context locals
		c.Locals("inputCreateDirector", input)
		c.Locals("avatarUrl", avatarUrl)
		// Continue to next handler
		return c.Next()
	}
}
func UpdateDirector(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền tạo đạo diễn", errors.New("permission denied"))
		}
		var input model.UpdateDirectorInput
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Dữ liệu không hợp lệ: %s", err.Error()),
			})
		}

		// Validate dữ liệu
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Kiểm tra diễn viên có tồn tại không
		var director model.Director
		if err := database.DB.First(&director, valueKey).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Không tìm thấy diễn viên",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %v", err),
			})
		}
		cld, err := cloudinary.NewFromParams(
			os.Getenv("CLOUDINARY_CLOUD_NAME"),
			os.Getenv("CLOUDINARY_API_KEY"),
			os.Getenv("CLOUDINARY_API_SECRET"))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Không thể khởi tạo Cloudinary: %s", err.Error()),
			})
		}
		var avatarUrl string
		if file, err := c.FormFile("avatar"); err == nil {
			// Kiểm tra định dạng file
			ext := filepath.Ext(file.Filename)
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Định dạng file không hỗ trợ (chỉ hỗ trợ PNG, JPG, JPEG)", fmt.Errorf("invalid file format"), "logo")
			}

			// Mở file
			fileReader, err := file.Open()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Không thể đọc file avatar: %s", err.Error()),
				})
			}
			defer fileReader.Close()
			// Xóa logo cũ nếu tồn tại
			if director.Avatar != nil && *director.Avatar != "" {
				publicID := helper.ExtractPublicID(*director.Avatar)
				if publicID != "" {
					_, err := cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
						PublicID:     publicID,
						ResourceType: "image",
					})
					if err != nil {
						log.Printf("Không thể xóa logo cũ trên Cloudinary: %s", err.Error())
						// Tiếp tục xử lý, không trả về lỗi
					}
				}
			}
			// Tải lên Cloudinary
			uploadResult, err := cld.Upload.Upload(context.Background(), fileReader, uploader.UploadParams{
				Folder:       "avatar",
				PublicID:     fmt.Sprintf("logo_%s_%d", director.Name, time.Now().Unix()),
				ResourceType: "image",
			})
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Không thể tải lên Cloudinary: %s", err.Error()),
				})
			}

			avatarUrl = uploadResult.SecureURL
		} else if input.Avatar != nil {
			// Xóa logo cũ nếu LogoUrl mới được cung cấp
			if director.Avatar != nil && *director.Avatar != "" {
				publicID := helper.ExtractPublicID(*director.Avatar)
				if publicID != "" {
					_, err := cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
						PublicID:     publicID,
						ResourceType: "image",
					})
					if err != nil {
						log.Printf("Không thể xóa logo cũ trên Cloudinary: %s", err.Error())
					}
				}
			}
			// Sử dụng LogoUrl từ input
			avatarUrl = *input.Avatar
		} else {
			// Giữ nguyên LogoUrl hiện tại nếu không có file hoặc logoUrl
			avatarUrl = *director.Avatar
		}
		c.Locals("directorId", valueKey)
		c.Locals("updateDirectorInput", input)
		c.Locals("avatarUrl", avatarUrl)
		return c.Next()
	}
}
func DeleteDirector(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Lấy ID
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}

		// 2. Kiểm tra quyền
		_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xóa đạo diễn", nil)
		}

		// 3. Tìm đạo diễn
		var director model.Director
		if err := database.DB.Unscoped().First(&director, valueKey).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrorResponse(c, fiber.StatusNotFound, "Đạo diễn không tồn tại hoặc đã bị xóa", nil)
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn", err)
		}

		// 4. Kiểm tra xem đạo diễn có đang được dùng trong phim không
		var movieCount int64
		if err := database.DB.Model(&model.Movie{}).
			Where("director_id = ? ", valueKey).
			Count(&movieCount).Error; err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi kiểm tra phim", err)
		}

		if movieCount > 0 {
			return utils.ErrorResponse(c, fiber.StatusConflict,
				fmt.Sprintf("Không thể xóa: Đạo diễn đang được dùng trong %d phim", movieCount), nil)
		}
		c.Locals("directorId", valueKey)
		return c.Next()
	}
}
