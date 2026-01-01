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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// func CreateCinemaChain() fiber.Handler {
// 	return func(c *fiber.Ctx) error {
// 		var input model.CreateCinemaChainInput
// 		if err := c.BodyParser(&input); err != nil {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": fmt.Sprintf("Không thể phân tích yêu cầu: %s", err.Error()),
// 			})
// 		}

// 		// Validate input
// 		validate := validator.New()
// 		if err := validate.Struct(&input); err != nil {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": err.Error(),
// 			})
// 		}
// 		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

// 		if !isAdmin {
// 			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
// 		}
// 		var existingChain model.CinemaChain
// 		if err := database.DB.Where("name = ?", input.Name).First(&existingChain).Error; err == nil {
// 			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên chuỗi rạp đã tồn tại", fmt.Errorf("name already exists"), "name")
// 		}

// 		// Khởi tạo Cloudinary
// 		cld, err := cloudinary.NewFromParams(
// 			os.Getenv("CLOUDINARY_CLOUD_NAME"),
// 			os.Getenv("CLOUDINARY_API_KEY"),
// 			os.Getenv("CLOUDINARY_API_SECRET"))
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": fmt.Sprintf("Không thể khởi tạo Cloudinary: %s", err.Error()),
// 			})
// 		}

// 		var logoUrl string
// 		if file, err := c.FormFile("logo"); err == nil {
// 			// Kiểm tra định dạng file
// 			ext := filepath.Ext(file.Filename)
// 			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
// 				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Định dạng file không hỗ trợ (chỉ hỗ trợ PNG, JPG, JPEG)", fmt.Errorf("invalid file format"), "logo")
// 			}
// 			// Mở file
// 			fileReader, err := file.Open()
// 			if err != nil {
// 				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"error": fmt.Sprintf("Không thể đọc file logo: %s", err.Error()),
// 				})
// 			}
// 			defer fileReader.Close()

// 			// Tải lên Cloudinary
// 			uploadResult, err := cld.Upload.Upload(context.Background(), fileReader, uploader.UploadParams{
// 				Folder:       "cinema_chains",
// 				PublicID:     fmt.Sprintf("logo_%s_%d", input.Name, time.Now().Unix()),
// 				ResourceType: "image",
// 			})
// 			if err != nil {
// 				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"error": fmt.Sprintf("Không thể tải lên Cloudinary: %s", err.Error()),
// 				})
// 			}

// 			logoUrl = uploadResult.SecureURL
// 			// // Tạo tên file duy nhất
// 			// filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
// 			// savePath := fmt.Sprintf("public/uploads/logos/%s", filename)

// 			// // Lưu file vào thư mục
// 			// if err := c.SaveFile(file, savePath); err != nil {
// 			// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			// 		"error": fmt.Sprintf("Không thể lưu file logo: %s", err.Error()),
// 			// 	})
// 			// }

//				// // Tạo URL cho logo
//				// logoUrl = fmt.Sprintf("/uploads/logos/%s", filename)
//			} else if input.Logo != "" {
//				// Sử dụng LogoUrl từ input nếu không có file
//				logoUrl = input.Logo
//			}
//			c.Locals("inputCreateCinemaChain", input)
//			c.Locals("logoUrl", logoUrl)
//			return c.Next()
//		}
//	}
func CreateCinemaChain() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Parse FormData (có file)
		form, err := c.MultipartForm()
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Không thể đọc form data", err)
		}
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}
		// 2. Lấy dữ liệu text
		name := form.Value["name"][0]
		description := form.Value["description"][0]
		active := form.Value["active"][0] == "1"

		// Validate
		if name == "" {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên chuỗi rạp không được để trống", nil, "name")
		}

		// Kiểm tra trùng tên
		var existingChain model.CinemaChain
		if err := database.DB.Where("name = ?", name).First(&existingChain).Error; err == nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên chuỗi rạp đã tồn tại", nil, "name")
		}
		input := model.CreateCinemaChainInput{
			Name:        name,
			Description: description,
			Active:      &active,
		}
		// 3. Xử lý file logo
		var logoUrl string
		if files := form.File["logo"]; len(files) > 0 {
			file := files[0]

			// Validate định dạng
			ext := strings.ToLower(filepath.Ext(file.Filename))
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Chỉ hỗ trợ PNG, JPG, JPEG", nil, "logo")
			}

			// Mở file
			fileReader, err := file.Open()
			if err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể đọc file", err)
			}
			defer fileReader.Close()

			// Upload Cloudinary
			cld, err := cloudinary.NewFromParams(
				os.Getenv("CLOUDINARY_CLOUD_NAME"),
				os.Getenv("CLOUDINARY_API_KEY"),
				os.Getenv("CLOUDINARY_API_SECRET"),
			)
			if err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi Cloudinary", err)
			}

			uploadResult, err := cld.Upload.Upload(context.Background(), fileReader, uploader.UploadParams{
				Folder:       "cinema_chains",
				PublicID:     fmt.Sprintf("logo_%s_%d", name, time.Now().Unix()),
				ResourceType: "image",
			})
			if err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Upload thất bại", err)
			}

			logoUrl = uploadResult.SecureURL
		}

		// 4. Tạo chain
		c.Locals("inputCreateCinemaChain", input)
		c.Locals("logoUrl", logoUrl)
		return c.Next()
	}
}
func EditCinemaChain(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Lấy ID
		id, err := strconv.Atoi(c.Params(key))
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, err)
		}

		// 2. Parse form
		form, err := c.MultipartForm()
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid form", err)
		}

		// 3. Lấy dữ liệu
		name := utils.GetFirstValue(form.Value, "name")
		description := utils.GetFirstValue(form.Value, "description")
		activeStr := utils.GetFirstValue(form.Value, "active")

		// 4. Tạo input
		var namePtr, descPtr *string
		var activePtr *bool

		if name != "" {
			namePtr = &name
		}
		if description != "" {
			descPtr = &description
		}
		if activeStr != "" {
			active := activeStr == "1"
			activePtr = &active
		}

		input := model.EditCinemaChainInput{
			Name:        namePtr,
			Description: descPtr,
			Active:      activePtr,
		}

		// 5. Validate partial
		validate := validator.New()
		if err := validate.StructPartial(input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, err.Error(), err)
		}

		// 6. Kiểm tra quyền admin
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}

		// 7. Tìm chain
		var chain model.CinemaChain
		if err := database.DB.First(&chain, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Không tìm thấy chuỗi rạp"})
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi DB", err)
		}

		// 8. Kiểm tra tên trùng (nếu thay đổi)
		if namePtr != nil && *namePtr != chain.Name {
			var existing model.CinemaChain
			if err := database.DB.Where("name = ? AND id != ?", *namePtr, id).First(&existing).Error; err == nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên đã tồn tại", nil, "name")
			}
		}

		// 9. Xử lý upload logo
		// 9️⃣ Xử lý upload logo
		logoUrl := chain.LogoUrl
		file, err := c.FormFile("logo")
		if err == nil && file != nil {
			// 🔹 Kiểm tra định dạng
			ext := strings.ToLower(filepath.Ext(file.Filename))
			if !slices.Contains([]string{".png", ".jpg", ".jpeg"}, ext) {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Chỉ hỗ trợ PNG, JPG, JPEG", nil, "logo")
			}

			// 🔹 Khởi tạo Cloudinary
			cld, err := cloudinary.NewFromParams(
				os.Getenv("CLOUDINARY_CLOUD_NAME"),
				os.Getenv("CLOUDINARY_API_KEY"),
				os.Getenv("CLOUDINARY_API_SECRET"),
			)
			if err != nil {
				return utils.ErrorResponse(c, 500, "Không thể khởi tạo Cloudinary", err)
			}

			// 🔹 Xóa ảnh cũ
			if chain.LogoUrl != "" {
				publicID := helper.ExtractPublicID(chain.LogoUrl)
				if publicID != "" {
					_, err := cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
						PublicID:     publicID,
						ResourceType: "image",
					})
					if err != nil {
						log.Printf("Không thể xóa logo cũ: %v", err)
					}
				}
			}

			// 🔹 Upload ảnh mới
			fileReader, err := file.Open()
			if err != nil {
				return utils.ErrorResponse(c, 500, "Không thể mở file", err)
			}
			defer fileReader.Close()

			result, err := cld.Upload.Upload(context.Background(), fileReader, uploader.UploadParams{
				Folder:   "cinema_chains",
				PublicID: fmt.Sprintf("logo_%d_%d", id, time.Now().UnixNano()),
			})
			if err != nil {
				return utils.ErrorResponse(c, 500, "Upload thất bại", err)
			}
			logoUrl = result.SecureURL
		}

		// 10. Lưu vào Locals
		c.Locals("inputEditCinemaChain", input)
		c.Locals("logoUrl", logoUrl)
		c.Locals("chainId", uint(id))
		return c.Next()
	}

}

func CreateCinema() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateCinemaInput
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
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}
		// Kiểm tra CinemaChain tồn tại
		var chain model.CinemaChain
		if err := database.DB.First(&chain, input.ChainId).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Chuỗi rạp chiếu phim không tồn tại", fmt.Errorf("chainId not found"), "chainId")
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}
		var existingCinema model.Cinema
		cinemaName := fmt.Sprintf("%s", strings.TrimSpace(input.Name))
		if err := database.DB.Where("chain_id = ? AND name = ?", input.ChainId, cinemaName).First(&existingCinema).Error; err == nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Cinema name already exists", errors.New("DUPLICATE_CINEMA_NAME"))
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Error checking existing cinema name: %v", err)
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
		}
		// Tạo tên rạp: <CinemaChain.Name> <Location>
		// Save input to context locals
		c.Locals("inputCreateCinema", input)
		c.Locals("cinemaName", cinemaName)

		// Continue to next handler
		return c.Next()
	}
}
func EditCinema(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		var input model.EditCinemaInput

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
		_, isAdmin, isQuanLy, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isQuanLy {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.CAN_NOT_EDIT_CINEMA, errors.New("not permission"))
		}
		// Kiểm tra CinemaChain tồn tại
		var chain model.CinemaChain
		if err := database.DB.First(&chain, input.ChainId).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Chuỗi rạp chiếu phim không tồn tại", fmt.Errorf("chainId not found"), "chainId")
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}
		// Kiểm tra tên rạp không trùng trong cùng chuỗi (trừ bản ghi hiện tại)
		// Check for duplicate cinema name within the same chain (excluding current cinema)
		// rawLocation := ""
		// if input.Name != nil {
		// 	rawLocation = *input.Name
		// }

		var cinemaName string
		if input.Name != nil {
			cinemaName = strings.TrimSpace(*input.Name)
		}
		if input.Name != nil {
			var existingCinema model.Cinema
			if err := database.DB.Where("name = ? AND chain_id = ? AND id != ?", cinemaName, input.ChainId, valueKey).First(&existingCinema).Error; err == nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Tên rạp đã tồn tại trong chuỗi này", fmt.Errorf("name already exists"), "name")
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, fmt.Errorf("database query error: %v", err))
			}
		}
		// Verify address if provided
		var lat, lng float64
		if input.Address != nil {
			if input.Address.Latitude != 0 && input.Address.Longitude != 0 {
				// Nếu front-end đã gửi tọa độ → dùng luôn
				lat = input.Address.Latitude
				lng = input.Address.Longitude
			} else {
				// Nếu không có → gọi verify
				lat, lng, err = helper.VerifyAddress(*input.Address)
				if err != nil {
					return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid address", err)
				}
			}
		}
		// Tạo tên rạp: <CinemaChain.Name> <Location>

		// Save input to context locals
		c.Locals("inputEditCinema", input)
		c.Locals("inputCinemaId", uint(valueKey))
		c.Locals("cinemaName", cinemaName)
		c.Locals("latitude", lat)
		c.Locals("longitude", lng)
		//log.Printf("Middleware completed: cinemaId=%d, cinemaName=%v, lat=%f, lng=%f", valueKey, c.Locals("cinemaName"), lat, lng)
		// Continue to next handler
		return c.Next()
	}
}
