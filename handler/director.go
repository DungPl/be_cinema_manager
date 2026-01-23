package handler

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
	"strconv"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gofiber/fiber/v2"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

func GetDirector(c *fiber.Ctx) error {
	_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isKiemDuyet {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem ", errors.New("permission denied"))
	}

	var directors []model.Director
	var total int64
	db := database.DB
	search := strings.TrimSpace(c.Query("search", ""))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	query := db.Model(&model.Director{})
	if search != "" {
		searchPattern := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(name) LIKE ?", searchPattern)
	}

	// Đếm tổng
	if err := query.Count(&total).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể đếm tổng số diễn viên", err)
	}

	// Lấy dữ liệu trang
	if err := query.
		Offset(offset).
		Limit(limit).
		Order("id DESC").
		Find(&directors).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể lấy danh sách diễn viên", err)
	}

	response := &model.ResponseCustom{
		Rows:       directors,
		Limit:      &limit,
		Page:       &page,
		TotalCount: total,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetDirectorById(c *fiber.Ctx) error {
	db := database.DB

	directorIdParam := c.Params("directorId")
	directorId, err := strconv.Atoi(directorIdParam)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "ID đạo diễn không hợp lệ", err)
	}
	_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isKiemDuyet {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem ", errors.New("permission denied"))
	}
	var director model.Director
	if err := db.
		First(&director, directorId).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy phim", err)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi khi truy vấn cơ sở dữ liệu", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, director)
}
func GetDirectorByMovieId(c *fiber.Ctx) error {
	db := database.DB

	movieIdParam := c.Params("inputId")
	movieId, err := strconv.Atoi(movieIdParam)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "ID phim không hợp lệ", err)
	}
	_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isKiemDuyet {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem ", errors.New("permission denied"))
	}

	var movie model.Movie
	if err := db.Preload("Director").
		First(&movie, movieId).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy phim", err)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi khi truy vấn cơ sở dữ liệu", err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movieId":  movie.ID,
		"title":    movie.Title,
		"director": movie.Directors,
	})
}

// handlers/director_handler.go
func CreateDirector(c *fiber.Ctx) error {

	db := database.DB
	input, ok := c.Locals("inputCreateDirector").(model.CreateDirectorInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	avatarUrl, ok := c.Locals("avatarUrl").(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy URL avatar",
		})
	}
	// Tạo mới
	tx := db.Begin()
	newDirector := new(model.Director)
	input.Avatar = &avatarUrl
	copier.Copy(&newDirector, &input)

	if err := database.DB.Create(&newDirector).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi khi tạo đạo diễn", err)
	}

	return utils.SuccessResponse(c, fiber.StatusCreated, newDirector)
}
func UpdateDirector(c *fiber.Ctx) error {
	directorId := c.Locals("directorId").(int)
	input := c.Locals("updateDirectorInput").(model.UpdateDirectorInput)
	avatarUrl := c.Locals("avatarUrl").(string)
	var director model.Director
	if err := database.DB.First(&director, directorId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Không tìm thấy diễn viên để cập nhật",
		})
	}
	if input.Name != nil {
		director.Name = *input.Name
	}
	// Cập nhật thông tin
	if input.Biography != nil {
		director.Biography = input.Biography
	}
	if input.Nationality != nil {
		director.Nationality = *input.Nationality
	}
	if input.DirectorUrl != nil {
		director.DirectorUrl = input.DirectorUrl
	}
	if avatarUrl != "" {
		director.Avatar = &avatarUrl
	}

	if err := database.DB.Save(&director).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể cập nhật diễn viên: %v", err),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Cập nhật thông tin đạo diễn  thành công",
		"actor":   director,
	})
}
func DeleteDirector(c *fiber.Ctx) error {
	arrayId := c.Locals("deleteIds").(model.ArrayId)
	ids := arrayId.IDs
	// 5. Lấy Cloudinary
	cld, err := cloudinary.NewFromParams(
		os.Getenv("CLOUDINARY_CLOUD_NAME"),
		os.Getenv("CLOUDINARY_API_KEY"),
		os.Getenv("CLOUDINARY_API_SECRET"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể khởi tạo Cloudinary: %s", err.Error()),
		})
	}
	for _, id := range ids {
		var director model.Director
		// 1. Load đạo diễn từ DB
		if err := database.DB.First(&director, id).Error; err != nil {
			// Nếu không tìm thấy, bỏ qua hoặc log
			log.Printf("Không tìm thấy đạo diễn với ID %d", id)
			continue
		}

		// 2. Xóa ảnh trên Cloudinary (nếu có)
		if director.Avatar != nil && *director.Avatar != "" {
			invalidate := true
			go func(publicID string) {
				_, err := cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
					PublicID:     publicID,
					ResourceType: "image",
					Invalidate:   &invalidate,
				})
				if err != nil {
					log.Printf("Failed to delete Cloudinary image %s: %v", publicID, err)
				}
			}(*director.Avatar)
		}

		// 3. Soft delete đạo diễn
		if err := database.DB.Delete(&director).Error; err != nil {
			log.Printf("Lỗi xóa đạo diễn ID %d: %v", id, err)
		}
	}

	// 4. Trả về response
	return utils.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"message": "Xóa đạo diễn thành công",
		"ids":     ids,
	})

}
