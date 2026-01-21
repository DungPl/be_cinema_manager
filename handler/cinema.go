package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

type CinemaWithRoomCount struct {
	model.Cinema
	RoomCount int64 `gorm:"column:room_count"`
}

func GetCinema(c *fiber.Ctx) error {
	filterInput := new(model.FilterCinema)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}

	db := database.DB
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.CAN_NOT_GET_CINEMA, errors.New("not permission"))
	}

	limit := 5
	page := 1

	if filterInput.Limit != nil && *filterInput.Limit > 0 {
		limit = *filterInput.Limit
		if limit > 500 {
			limit = 500
		}
	}
	if filterInput.Page != nil && *filterInput.Page > 0 {
		page = *filterInput.Page
	}

	offset := (page - 1) * limit

	key := strings.ToLower(strings.TrimSpace(filterInput.SearchKey))
	key = strings.NewReplacer(
		"q.", "quận", "h.", "huyện", "p.", "phường",
		"tp.", "thành phố", "hn", "hà nội", "hcm", "hồ chí minh",
		"sg", "hồ chí minh", "dn", "đà nẵng",
	).Replace(key)

	// === QUERY GỐC (KHÔNG LIMIT/OFFSET) ===
	baseQuery := db.Model(&model.Cinema{}).
		Select("cinemas.*, COALESCE(COUNT(DISTINCT rooms.id), 0) AS room_count").
		Joins("JOIN addresses ON addresses.cinema_id = cinemas.id").
		Joins("JOIN cinema_chains ON cinema_chains.id = cinemas.chain_id").
		Joins("LEFT JOIN rooms ON rooms.cinema_id = cinemas.id").
		Where("cinemas.active = ?", true)

	if isManager {
		baseQuery = baseQuery.Where("cinemas.id = ?", accountInfo.CinemaId)
	}
	if filterInput.SearchKey != "" {
		search := "%" + key + "%"
		baseQuery = baseQuery.Where(
			db.Where("LOWER(cinemas.name) LIKE ?", search).
				Or("LOWER(cinemas.phone) LIKE ?", search).
				Or("LOWER(addresses.province) LIKE ?", search).
				Or("LOWER(addresses.district) LIKE ?", search).
				Or("LOWER(addresses.full_address) LIKE ?", search).
				Or("LOWER(cinema_chains.name) LIKE ?", search),
		)
	}
	if filterInput.Province != "" {
		baseQuery = baseQuery.Where("LOWER(addresses.province) LIKE ?", "%"+strings.ToLower(filterInput.Province)+"%")
	}
	if filterInput.District != "" {
		baseQuery = baseQuery.Where("LOWER(addresses.district) LIKE ?", "%"+strings.ToLower(filterInput.District)+"%")
	}
	if filterInput.ChainName != "" {
		baseQuery = baseQuery.Where("LOWER(cinema_chains.name) LIKE ?", "%"+strings.ToLower(filterInput.ChainName)+"%")
	}
	if filterInput.ChainId != 0 {
		baseQuery = baseQuery.Where("cinemas.chain_id = ?", filterInput.ChainId)
	}

	// === TẠO QUERY ĐẾM (KHÔNG LIMIT/OFFSET) ===
	var totalCount int64
	countQuery := baseQuery.Session(&gorm.Session{}) // clone sạch
	if err := countQuery.Group("cinemas.id").Count(&totalCount).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Count failed", err)
	}

	// === QUERY LẤY DỮ LIỆU (CÓ LIMIT/OFFSET) ===
	var cinemas []CinemaWithRoomCount
	err := baseQuery.
		Group("cinemas.id").
		Offset(offset).
		Limit(limit).
		Order("cinemas.id DESC").
		Preload("Chain").
		Preload("Addresses").
		Preload("Promotions", func(db *gorm.DB) *gorm.DB {
			return db.Where("end_date >= ?", time.Now())
		}).
		Find(&cinemas).Error

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Query failed", err)
	}

	var result []model.Cinema
	for _, item := range cinemas {
		cinema := item.Cinema
		cinema.RoomCount = item.RoomCount
		cinema.Rooms = nil
		result = append(result, cinema)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, &model.ResponseCustom{
		Rows:       result,
		Limit:      &limit,
		Page:       &page,
		TotalCount: totalCount,
	})
}

func GetCinemaById(c *fiber.Ctx) error {
	db := database.DB

	param := c.Params("cinemaId")
	id, err := strconv.Atoi(param)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID rạp không hợp lệ",
		})
	}
	cinemaId := uint(id)

	var cinema model.Cinema
	var address model.Address

	// Tìm address (nếu có)
	if err := db.Where("cinema_id = ?", cinemaId).First(&address).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, fmt.Errorf("lỗi truy vấn cơ sở dữ liệu: %v", err))
		}
		log.Printf("No address found for cinemaId=%d", cinemaId)
	}
	var roomCount int64
	db.Model(&model.Room{}).Where("cinema_id = ?", cinemaId).Count(&roomCount)
	// Preload Chain và Addresses
	db.Preload("Chain").Preload("Addresses").First(&cinema, cinemaId)

	// Lấy thông tin user từ token
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

	// Sửa: Cho phép nếu là Admin HOẶC Manager
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.CAN_NOT_EDIT_CINEMA, errors.New("not permission"))
	}

	// Nếu là Manager → kiểm tra rạp có phải của họ không
	if isManager {
		if accountInfo.CinemaId == nil || *accountInfo.CinemaId != cinemaId {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem phòng của rạp khác", errors.New("cinema permission denied"))
		}
	}

	// Trả về cinema
	response := struct {
		*model.Cinema
		RoomCount int64 `json:"roomCount"`
	}{
		Cinema:    &cinema,
		RoomCount: roomCount,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func CreateCinemaChain(c *fiber.Ctx) error {
	cinemaChainInput, ok := c.Locals("inputCreateCinemaChain").(model.CreateCinemaChainInput)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy dữ liệu đầu vào",
		})
	}
	logoUrl, ok := c.Locals("logoUrl").(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy URL logo",
		})
	}
	db := database.DB
	tx := db.Begin()
	chain := model.CinemaChain{
		Name:        cinemaChainInput.Name,
		Description: cinemaChainInput.Description,
		LogoUrl:     logoUrl,
	}
	if chain.Active == nil {
		active := true
		chain.Active = &active
	}
	if err := tx.Create(&chain).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.ERROR_INTERNAL_ERROR, err)
	}
	tx.Commit()
	return utils.SuccessResponse(c, fiber.StatusOK, chain)
}
func EditCinemaChain(c *fiber.Ctx) error {
	input, ok := c.Locals("inputEditCinemaChain").(model.EditCinemaChainInput)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy dữ liệu đầu vào",
		})
	}
	logoUrl, ok := c.Locals("logoUrl").(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy URL logo",
		})
	}
	chainId, ok := c.Locals("chainId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy ID chuỗi rạp",
		})
	}

	db := database.DB
	var chain model.CinemaChain
	if err := db.First(&chain, chainId).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy chuỗi rạp", err)
	}

	// Chỉ cập nhật field nào có thay đổi
	updateData := map[string]interface{}{}

	if input.Name != nil {
		updateData["name"] = *input.Name
	}
	if input.Description != nil {
		updateData["description"] = *input.Description
	}
	if input.Active != nil {
		updateData["active"] = *input.Active
	}
	if logoUrl != "" && logoUrl != chain.LogoUrl {
		updateData["logo_url"] = logoUrl
	}

	if len(updateData) == 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Không có dữ liệu để cập nhật", nil)
	}

	// Cập nhật trong DB
	if err := db.Model(&chain).Updates(updateData).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể cập nhật chuỗi rạp", err)
	}

	// Lấy lại dữ liệu sau khi update
	if err := db.First(&chain, chainId).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể lấy thông tin sau cập nhật", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, chain)
}
func GetCinemaChain(c *fiber.Ctx) error {
	filterInput := new(model.FilterCinemaChain)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}

	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
	}

	db := database.DB

	condition := db.Model(&model.CinemaChain{})

	if filterInput.SearchKey != "" {
		condition = condition.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(filterInput.SearchKey)+"%")
	}

	var totalCount int64
	condition.Count(&totalCount)

	condition = utils.ApplyPagination(condition, filterInput.Limit, filterInput.Page)

	var cinemaChains []model.CinemaChain
	if err := condition.Find(&cinemaChains).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}

	response := &model.ResponseCustom{
		Rows:       cinemaChains,
		Limit:      filterInput.Limit,
		Page:       filterInput.Page,
		TotalCount: totalCount,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetCinemaChainById(c *fiber.Ctx) error {
	db := database.DB

	chainIdStr := c.Params("chainId")
	chainId, err := strconv.ParseUint(chainIdStr, 10, 64)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "ID chuỗi rạp không hợp lệ", err)
	}
	var chain model.CinemaChain
	if err := db.Preload("Cinemas").First(&chain, chainId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy chuỗi rạp", err)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi khi truy vấn dữ liệu chuỗi rạp", err)
	}

	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.ACCOUNT_NOT_PERMISSION, errors.New("account not permission"))
	}

	return utils.SuccessResponse(c, fiber.StatusOK, chain)

}
func DeleteCinamaChain(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
	}
	db := database.DB
	arrayId := c.Locals("deleteIds").(model.ArrayId)
	ids := arrayId.IDs
	if err := db.Model(&model.CinemaChain{}).Where("id in ?", ids).Update("Active", false).Error; err != nil {
		fmt.Println("Error:", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, ids)
}
func CreateCinema(c *fiber.Ctx) error {
	// Lấy dữ liệu từ context locals
	input, ok := c.Locals("inputCreateCinema").(model.CreateCinemaInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA CINEMA TO LOCALS FAIL"))
	}
	cinemaName, ok := c.Locals("cinemaName").(string)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA CINEMA TO LOCALS FAIL"))
	}
	// Verify address and get coordinates
	db := database.DB
	tx := db.Begin()

	tx.Preload("CinemaChain")
	newCinema := new(model.Cinema)
	copier.Copy(&newCinema, input)
	newCinema.Name = cinemaName

	if newCinema.Active == nil {
		active := true
		newCinema.Active = &active
	}
	newCinema.Slug = helper.GenerateUniqueCinemaSlug(tx, cinemaName)
	if err := tx.Preload("CinemaChain").Create(&newCinema).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_CREATE, err)
	}

	newAddress := model.Address{
		CinemaId:    newCinema.ID,
		Province:    input.Address.Province,
		District:    input.Address.District,
		Street:      input.Address.Street,
		Ward:        input.Address.Ward,
		HouseNumber: input.Address.HouseNumber,
		Latitude:    input.Address.Latitude,
		Longitude:   input.Address.Longitude,
		FullAddress: strings.TrimSpace(input.Address.FullAddress), // Ưu tiên từ frontend (đã đẹp từ display)
	}

	// Nếu frontend không gửi fullAddress → tự generate fallback
	if newAddress.FullAddress == "" {
		parts := []string{}
		if input.Address.HouseNumber != nil {
			parts = append(parts, strings.TrimSpace(*input.Address.HouseNumber))
		}
		if input.Address.Street != nil {
			parts = append(parts, strings.TrimSpace(*input.Address.Street))
		}
		if input.Address.Ward != nil {
			parts = append(parts, strings.TrimSpace(*input.Address.Ward))
		}
		if input.Address.District != nil {
			parts = append(parts, strings.TrimSpace(*input.Address.District))
		}
		if input.Address.Province != nil {
			parts = append(parts, strings.TrimSpace(*input.Address.Province))
		}
		if len(parts) > 0 {
			newAddress.FullAddress = strings.Join(parts, ", ") + ", Việt Nam"
		} else {
			newAddress.FullAddress = "Việt Nam"
		}
	}

	if err := tx.Create(&newAddress).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_CREATE_ADDRESS, err)
	}

	tx.Commit()
	return utils.SuccessResponse(c, fiber.StatusOK, newCinema)
}
func EditCinema(c *fiber.Ctx) error {
	db := database.DB
	cinemaInput, ok := c.Locals("inputEditCinema").(model.EditCinemaInput)

	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA CINEMA TO LOCALS FAIL"))
	}

	cinemaName, ok := c.Locals("cinemaName").(string)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA CINEMA NAME TO LOCALS FAIL"))
	}
	cinemaId, ok := c.Locals("inputCinemaId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy ID rạp",
		})
	}

	tx := db.Begin()
	var cinema model.Cinema
	tx.First(&cinema, cinemaId)
	if cinemaInput.Phone != nil {
		cinema.Phone = *cinemaInput.Phone
	}
	if cinemaInput.Name != nil {
		cinema.Name = cinemaName // CHỈ LÀ TÊN RẠP CON
	}
	cinema.Slug = helper.GenerateUniqueCinemaSlug(tx, cinemaName)
	if cinemaInput.Active != nil {
		*cinema.Active = *cinemaInput.Active
	}
	if cinemaInput.Description != nil {
		cinema.Description = cinemaInput.Description
	}
	if cinemaInput.ChainId != nil {
		cinema.ChainId = *cinemaInput.ChainId
	}
	if err := tx.Save(&cinema).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_EDIT, err)
	}

	// Update address if provided
	if cinemaInput.Address != nil {
		var address model.Address

		// Tìm address hiện có
		err := tx.Where("cinema_id = ?", cinemaId).First(&address).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn địa chỉ", err)
		}

		// Nếu chưa có → tạo mới
		if errors.Is(err, gorm.ErrRecordNotFound) {
			address = model.Address{CinemaId: uint(cinemaId)}
		}

		// Cập nhật các field
		if cinemaInput.Address.FullAddress != "" {
			address.FullAddress = strings.TrimSpace(cinemaInput.Address.FullAddress)
		}
		if cinemaInput.Address.Street != nil {
			address.Street = cinemaInput.Address.Street
		}
		if cinemaInput.Address.Ward != nil {
			address.Ward = cinemaInput.Address.Ward
		}
		if cinemaInput.Address.District != nil {
			address.District = cinemaInput.Address.District
		}
		if cinemaInput.Address.Province != nil {
			address.Province = cinemaInput.Address.Province
		}
		if cinemaInput.Address.HouseNumber != nil {
			address.HouseNumber = cinemaInput.Address.HouseNumber
		}

		// Bắt buộc phải có tọa độ
		// if cinemaInput.Address.Latitude == "" || cinemaInput.Address.Longitude == nil {
		// 	tx.Rollback()
		// 	return utils.ErrorResponse(c, fiber.StatusBadRequest, "Thiếu tọa độ khi cập nhật địa chỉ", errors.New("latitude/longitude required"))
		// }
		address.Latitude = cinemaInput.Address.Latitude
		address.Longitude = cinemaInput.Address.Longitude

		// Nếu fullAddress rỗng → tự generate
		if address.FullAddress == "" {
			parts := []string{}
			if address.HouseNumber != nil {
				parts = append(parts, strings.TrimSpace(*address.HouseNumber))
			}
			if address.Street != nil {
				parts = append(parts, strings.TrimSpace(*address.Street))
			}
			if address.Ward != nil {
				parts = append(parts, strings.TrimSpace(*address.Ward))
			}
			if address.District != nil {
				parts = append(parts, strings.TrimSpace(*address.District))
			}
			if address.Province != nil {
				parts = append(parts, strings.TrimSpace(*address.Province))
			}
			if len(parts) > 0 {
				address.FullAddress = strings.Join(parts, ", ") + ", Việt Nam"
			}
		}

		if address.ID == 0 {
			if err := tx.Create(&address).Error; err != nil {
				tx.Rollback()
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tạo địa chỉ", err)
			}
		} else {
			if err := tx.Save(&address).Error; err != nil {
				tx.Rollback()
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi cập nhật địa chỉ", err)
			}
		}
	}
	tx.Commit()
	// Load updated cinema and address for response
	var updatedCinema model.Cinema
	err := db.
		Preload("Chain").
		Preload("Addresses"). // ← THÊM DÒNG NÀY!
		Preload("Rooms").
		First(&updatedCinema, cinemaId).Error

	if err != nil {
		log.Printf("Failed to load updated cinema with address: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tải dữ liệu rạp sau khi cập nhật", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, updatedCinema)
}
func DeleteCinema(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.CAN_NOT_EDIT_CINEMA, errors.New("not permission"))
	}
	db := database.DB
	arrayId := c.Locals("deleteIds").(model.ArrayId)
	ids := arrayId.IDs

	if err := db.Model(&model.Cinema{}).Where("id in ? and active != true", ids).Update("active", constants.STATUS_CINEMA_CANCEL).Error; err != nil {
		fmt.Println("Error:", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, ids)
}
func GetCinemaProvinces(c *fiber.Ctx) error {
	var provinces []string

	err := database.DB.
		Model(&model.Address{}).
		Select("DISTINCT addresses.province").
		Joins("JOIN cinemas ON cinemas.id = addresses.cinema_id").
		Where("cinemas.active = ?", true).
		Order("addresses.province ASC").
		Pluck("addresses.province", &provinces).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Không lấy được danh sách tỉnh", err)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"rows": provinces,
	})
}
func GetCinemaDetail(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return utils.ErrorResponse(c, 400, "slug is required", nil)
	}

	var cinema model.Cinema

	err := database.DB.
		Preload("Addresses").
		Preload("Chain").
		Where("slug = ?", slug).
		First(&cinema).Error

	if err != nil {
		return utils.ErrorResponse(c, 404, "Cinema not found", err)
	}

	return utils.SuccessResponse(c, 200, cinema)
}
func GetCinemasByProvince(c *fiber.Ctx) error {
	province := c.Query("province")

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	searchKey := c.Query("searchKey")
	offset := (page - 1) * limit

	var cinemas []model.Cinema
	var total int64

	query := database.DB.
		Model(&model.Cinema{}).
		Where("cinemas.active = ?", true)

	if searchKey != "" {
		query = query.Where("cinemas.name ILIKE ?", "%"+searchKey+"%")
	}
	if province != "" && province != "all" {
		query = query.
			Joins("JOIN addresses ON addresses.cinema_id = cinemas.id").
			Where("addresses.province = ?", province)
	}

	query.Count(&total)

	err := query.
		Preload("Addresses").
		Preload("Chain").
		Preload("Rooms").
		Limit(limit).
		Offset(offset).
		Find(&cinemas).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Không lấy được danh sách rạp", err)
	}

	for i := range cinemas {
		cinemas[i].RoomCount = int64(len(cinemas[i].Rooms))
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"rows":  cinemas,
		"total": total,
	})
}

func GetShowtimeByCinemaId(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return utils.ErrorResponse(c, 400, "slug is required", nil)
	}

	db := database.DB
	dateStr := c.Query("date")

	var selectedDate time.Time
	if dateStr == "" {
		selectedDate = time.Now()
	} else {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return utils.ErrorResponse(c, 400, "date format must YYYY-MM-DD", err)
		}
		selectedDate = parsed
	}

	startOfDay := time.Date(
		selectedDate.Year(),
		selectedDate.Month(),
		selectedDate.Day(),
		0, 0, 0, 0,
		time.Local,
	)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var showtimes []model.Showtime
	err := db.
		Preload("Movie").
		Preload("Movie.Posters").
		Preload("Movie.Trailers").
		Preload("Room").
		Joins("JOIN rooms ON rooms.id = showtimes.room_id").
		Joins("JOIN cinemas ON cinemas.id = rooms.cinema_id").
		Where("cinemas.slug = ?", slug).
		Where("showtimes.start_time >= ? AND showtimes.start_time < ?", startOfDay, endOfDay).
		Order("showtimes.start_time ASC").
		Find(&showtimes).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Không lấy được suất chiếu", err)
	}

	type MovieWithShowtimes struct {
		Movie     model.Movie      `json:"movie"`
		Showtimes []model.Showtime `json:"showtimes"`
	}

	movieMap := make(map[uint]*MovieWithShowtimes)

	for _, s := range showtimes {
		if movieMap[s.MovieId] == nil {
			movieMap[s.MovieId] = &MovieWithShowtimes{
				Movie:     s.Movie,
				Showtimes: []model.Showtime{},
			}
		}
		movieMap[s.MovieId].Showtimes = append(movieMap[s.MovieId].Showtimes, s)
	}

	result := make([]MovieWithShowtimes, 0, len(movieMap))
	for _, v := range movieMap {
		result = append(result, *v)
	}

	return utils.SuccessResponse(c, 200, result)
}

type CinemaChainWithCount struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	CinemaCount int    `json:"cinemaCount"`
}
type ProvinceChainSummary struct {
	Province     string                 `json:"province"`
	TotalCinemas int                    `json:"totalCinemas"`
	Chains       []CinemaChainWithCount `json:"chains"`
}

func GetProvincesWithChains(c *fiber.Ctx) error {
	db := database.DB

	type ProvinceResult struct {
		Province     string
		TotalCinemas int `gorm:"column:cinema_count"`
	}

	var provinceResults []ProvinceResult
	if err := db.Model(&model.Address{}).
		Select("province, count(distinct cinema_id) as cinema_count").
		Group("province").
		Find(&provinceResults).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Failed to fetch provinces", err)
	}

	// Sort provinces by total cinemas descending (adjust sorting if needed, e.g., alphabetical by province).
	sort.Slice(provinceResults, func(i, j int) bool {
		return provinceResults[i].TotalCinemas > provinceResults[j].TotalCinemas
	})

	var summaries []ProvinceChainSummary
	for _, p := range provinceResults {
		type ChainResult struct {
			ID          uint
			Name        string
			LogoUrl     string
			CinemaCount int `gorm:"column:cinema_count"`
		}

		var chainResults []ChainResult
		if err := db.Table("cinema_chains").
			Select("cinema_chains.id, cinema_chains.name, cinema_chains.logo_url, count(distinct cinemas.id) as cinema_count").
			Joins("join cinemas on cinemas.chain_id = cinema_chains.id").
			Joins("join addresses on addresses.cinema_id = cinemas.id").
			Where("addresses.province = ?", p.Province).
			Group("cinema_chains.id").
			Order("cinema_chains.name asc").
			Scan(&chainResults).Error; err != nil {
			return utils.ErrorResponse(c, 500, "Failed to fetch chains for province", err)
		}

		var chains []CinemaChainWithCount
		for _, ch := range chainResults {
			chains = append(chains, CinemaChainWithCount{
				ID:          ch.ID,
				Name:        ch.Name,
				CinemaCount: ch.CinemaCount,
			})
		}

		summaries = append(summaries, ProvinceChainSummary{
			Province:     p.Province,
			TotalCinemas: p.TotalCinemas,
			Chains:       chains,
		})
	}

	return utils.SuccessResponse(c, 200, summaries)
}
func GetCinemaChainsByArea(c *fiber.Ctx) error {
	db := database.DB
	province := c.Query("province")

	type Result struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CinemaCount int    `json:"cinemaCount"`
		LogoUrl     string `json:"logoUrl"`
	}

	tx := db.Table("cinema_chains").
		Select("cinema_chains.id,cinema_chains.logo_url ,cinema_chains.name, COUNT(cinemas.id) as cinema_count").
		Joins("JOIN cinemas ON cinemas.chain_id = cinema_chains.id").
		Joins("JOIN addresses ON addresses.cinema_id = cinemas.id")

	if province != "" {
		tx = tx.Where("addresses.province = ?", province)
	}

	tx = tx.Group("cinema_chains.id")

	var results []Result
	if err := tx.Scan(&results).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Cannot fetch cinema chains", err)
	}

	return utils.SuccessResponse(c, 200, results)
}
func GetCinemas(c *fiber.Ctx) error {
	db := database.DB

	chainId := c.QueryInt("chainId")
	province := c.Query("province")

	if chainId == 0 {
		return utils.ErrorResponse(c, 400, "chainId is required", nil)
	}

	tx := db.Model(&model.Cinema{}).
		Preload("Addresses").
		Where("chain_id = ?", chainId)

	if province != "" {
		tx = tx.Joins("JOIN addresses ON addresses.cinema_id = cinemas.id").
			Where("addresses.province = ?", province)
	}

	var cinemas []model.Cinema
	if err := tx.Find(&cinemas).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Cannot fetch cinemas", err)
	}

	return utils.SuccessResponse(c, 200, cinemas)
}
func SearchCinemas(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return utils.ErrorResponse(c, 400, "Thiếu từ khóa tìm kiếm", nil)
	}

	var cinemas []model.Cinema
	if err := database.DB.
		Where("name ILIKE ? ", "%"+query+"%").
		Find(&cinemas).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi tìm kiếm rạp", err)
	}

	return utils.SuccessResponse(c, 200, cinemas)
}
func GetShowtimes(c *fiber.Ctx) error {
	db := database.DB

	cinemaId := c.QueryInt("cinemaId")
	dateStr := c.Query("date")

	if cinemaId == 0 {
		return utils.ErrorResponse(c, 400, "cinemaId is required", nil)
	}

	tx := db.Model(&model.Showtime{}).
		Preload("Movie").
		Preload("Movie.Posters").
		Preload("Movie.Trailers").
		Preload("Room").
		Joins("JOIN rooms ON rooms.id = showtimes.room_id").
		Where("rooms.cinema_id = ?", cinemaId).
		Where("showtimes.status = ?", "AVAILABLE")

	// Lọc theo ngày (KHÔNG start_date)
	if dateStr != "" {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return utils.ErrorResponse(c, 400, "Invalid date format", err)
		}

		start := date
		end := date.Add(24 * time.Hour)

		tx = tx.Where("showtimes.start_time >= ? AND showtimes.start_time < ?", start, end)
	}

	tx = tx.Order("showtimes.start_time ASC")

	var showtimes []model.Showtime
	if err := tx.Find(&showtimes).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Cannot fetch showtimes", err)
	}

	return utils.SuccessResponse(c, 200, showtimes)
}
