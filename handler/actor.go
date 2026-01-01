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

	"github.com/gofiber/fiber/v2"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

func GetActor(c *fiber.Ctx) error {
	_, isAdmin, _, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isKiemDuyet {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem ", errors.New("permission denied"))
	}

	var actors []model.Actor
	var total int64
	db := database.DB
	search := strings.TrimSpace(c.Query("search", ""))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	query := db.Model(&model.Actor{})
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
		Find(&actors).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể lấy danh sách diễn viên", err)
	}

	response := &model.ResponseCustom{
		Rows:       actors,
		Limit:      &limit,
		Page:       &page,
		TotalCount: total,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetActorsByMovieId(c *fiber.Ctx) error {
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
	if err := db.Preload("Actors").
		First(&movie, movieId).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy phim", err)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi khi truy vấn cơ sở dữ liệu", err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movieId": movie.ID,
		"title":   movie.Title,
		"actors":  movie.Actors,
	})
}

func CreateActor(c *fiber.Ctx) error {

	db := database.DB
	input, ok := c.Locals("inputCreateActor").(model.Actor)
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
	newActor := new(model.Actor)

	copier.Copy(&newActor, &input)
	input.Avatar = &avatarUrl
	if err := database.DB.Create(&newActor).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi khi tạo đạo diễn", err)
	}

	return utils.SuccessResponse(c, fiber.StatusCreated, newActor)
}
func CreateActors(c *fiber.Ctx) error {
	input, ok := c.Locals("createActorsInput").(model.CreateActorsInput)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy dữ liệu đầu vào",
		})
	}

	var actors []model.Actor
	for _, name := range input.Names {
		actors = append(actors, model.Actor{
			Name: name,
		})
	}

	if err := database.DB.Create(&actors).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể tạo diễn viên: %v", err),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Tạo danh sách diễn viên thành công",
		"count":   len(actors),
		"actors":  actors,
	})
}
func UpdateActor(c *fiber.Ctx) error {
	actorId := c.Locals("actorId").(int)
	input := c.Locals("updateActorInput").(model.UpdateActorInput)
	avatarUrl := c.Locals("avatarUrl").(string)
	var actor model.Actor
	if err := database.DB.First(&actor, actorId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Không tìm thấy diễn viên để cập nhật",
		})
	}

	// 1. Cập nhật tên (kiểm tra trùng)
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed != actor.Name {
			var existing model.Actor
			if err := database.DB.
				Where("LOWER(name) = LOWER(?) AND id != ?", trimmed, actorId).
				First(&existing).Error; err == nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
					"Tên diễn viên đã tồn tại", nil, "name")
			}
			actor.Name = trimmed
		}
	}

	// 2. Cập nhật các trường khác

	if input.Nationality != nil {
		actor.Nationality = input.Nationality
	}

	// 3. CẬP NHẬT AVATAR
	if avatarUrl != "" {
		actor.Avatar = &avatarUrl
	}
	if input.Biography != nil {
		actor.Biography = input.Biography
	}
	if input.ActorUrl != nil {
		actor.ActorUrl = input.ActorUrl
	}
	if err := database.DB.Save(&actor).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể cập nhật diễn viên: %v", err),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Cập nhật thông tin diễn viên thành công",
		"actor":   actor,
	})
}
