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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func CreateActor() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.Actor

		// Parse body
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Dữ liệu không hợp lệ",
			})
		}

		// Validate
		if err := validate.Struct(input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, err.Error(), err)
		}

		// Kiểm tra quyền: chỉ admin hoặc kiểm duyệt
		_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isKiemDuyet {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền tạo đạo diễn", errors.New("permission denied"))
		}

		// Kiểm tra tên trùng (không phân biệt hoa thường)
		var existing model.Actor
		if err := database.DB.Where("LOWER(name) = LOWER(?)", input.Name).First(&existing).Error; err == nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên đạo diễn đã tồn tại", errors.New("name already exists"), "name")
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

			// Tải lên Cloudinary
			uploadResult, err := cld.Upload.Upload(context.Background(), fileReader, uploader.UploadParams{
				Folder:       "avatar_actor",
				PublicID:     fmt.Sprintf("avatar_%s_%d", input.Name, time.Now().Unix()),
				ResourceType: "image",
			})
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Không thể tải lên Cloudinary: %s", err.Error()),
				})
			}

			avatarUrl = uploadResult.SecureURL
		} else if *input.Avatar != "" {

			avatarUrl = *input.Avatar
		}
		// Save input to context locals
		c.Locals("inputCreateActor", input)
		c.Locals("avatarUrl", avatarUrl)
		// Continue to next handler
		return c.Next()
	}
}
func CreateActors(c *fiber.Ctx) error {
	var input model.CreateActorsInput

	// Parse JSON
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Dữ liệu không hợp lệ",
			"msg":   err.Error(),
		})
	}

	// Validate cơ bản
	if err := validate.Struct(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Dữ liệu không hợp lệ",
			"msg":   err.Error(),
		})
	}
	_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isKiemDuyet {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền tạo đạo diễn", errors.New("permission denied"))
	}
	// Kiểm tra trùng trong input
	nameSet := make(map[string]bool)
	for _, name := range input.Names {
		if nameSet[name] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Tên diễn viên '%s' bị trùng trong danh sách gửi lên", name),
			})
		}
		nameSet[name] = true
	}

	// Kiểm tra trùng trong DB
	var existing []model.Actor
	if err := database.DB.Where("name IN ?", input.Names).Find(&existing).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %v", err),
		})
	}

	if len(existing) > 0 {
		existingNames := []string{}
		for _, a := range existing {
			existingNames = append(existingNames, a.Name)
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Một số diễn viên đã tồn tại: %v", existingNames),
		})
	}

	// Lưu input vào context cho handler
	c.Locals("createActorsInput", input)
	return c.Next()
}
func UpdateActor(key string) fiber.Handler {
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
		// Lấy dữ liệu từ form
		name := strings.TrimSpace(c.FormValue("name"))
		nationality := strings.TrimSpace(c.FormValue("nationality"))
		biography := strings.TrimSpace(c.FormValue("biography"))
		actorUrl := strings.TrimSpace(c.FormValue("actorUrl"))
		input := model.UpdateActorInput{
			Name:        &name,
			Nationality: &nationality,
			Biography:   &biography,
			ActorUrl:    &actorUrl,
		}
		// Validate
		if name != "" && (len(name) < 2 || len(name) > 255) {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên phải từ 2-255 ký tự", nil, "name")
		}
		// Kiểm tra diễn viên có tồn tại không
		var actor model.Actor
		if err := database.DB.First(&actor, valueKey).Error; err != nil {
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
		// 7. Xử lý Avatar
		var avatarUrl string
		deleteOld := false

		// 1. Xử lý file
		if file, err := c.FormFile("avatar"); err == nil {
			ext := strings.ToLower(filepath.Ext(file.Filename))
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Chỉ hỗ trợ PNG, JPG, JPEG", nil, "avatar")
			}

			f, err := file.Open()
			if err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể đọc file", err)
			}
			defer f.Close()

			uploadResult, err := cld.Upload.Upload(c.Context(), f, uploader.UploadParams{
				Folder:   "actors",
				PublicID: fmt.Sprintf("actor_%d_%d", valueKey, time.Now().UnixNano()),
			})
			if err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Upload thất bại", err)
			}

			avatarUrl = uploadResult.SecureURL
			deleteOld = true
		}
		invalidate := true
		// 2. Xóa ảnh cũ
		if deleteOld && actor.Avatar != nil && *actor.Avatar != "" {
			publicID := helper.ExtractPublicID(*actor.Avatar)
			if publicID != "" {
				go cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
					PublicID:     publicID,
					ResourceType: "image",
					Invalidate:   &invalidate,
				})
			}
		}
		c.Locals("actorId", valueKey)
		c.Locals("updateActorInput", input)
		c.Locals("avatarUrl", avatarUrl)
		return c.Next()
	}
}
