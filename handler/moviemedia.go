package handler

import (
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"

	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func GenerateSignature(c *fiber.Ctx) error {
	_, isAdmin, isQuanLy, isKiemDuyet, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isQuanLy && !isKiemDuyet {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
	}

	type SigParams struct {
		Folder       string `json:"folder"`
		PublicID     string `json:"public_id"`
		ResourceType string `json:"resource_type"` // Parse but don't sign
	}

	var params SigParams
	if err := c.BodyParser(&params); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Params không hợp lệ", err)
	}

	timestamp := time.Now().Unix()
	timestampStr := fmt.Sprintf("%d", timestamp)

	// Collect signable parameters as map (exclude resource_type, api_key, etc.)
	paramMap := make(map[string]string)
	if params.Folder != "" {
		paramMap["folder"] = params.Folder // Raw value, no escape
	}
	if params.PublicID != "" {
		paramMap["public_id"] = params.PublicID // Raw value
	}
	paramMap["timestamp"] = timestampStr // Always include

	// Sort keys alphabetically
	keys := make([]string, 0, len(paramMap))
	for k := range paramMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build stringToSign manually (raw values, no URL encoding)
	var stringToSign strings.Builder
	for i, k := range keys {
		if i > 0 {
			stringToSign.WriteString("&")
		}
		stringToSign.WriteString(k)
		stringToSign.WriteString("=")
		stringToSign.WriteString(paramMap[k])
	}
	stringToSign.WriteString(os.Getenv("CLOUDINARY_API_SECRET"))

	// Compute SHA1 hash
	h := sha1.New()
	h.Write([]byte(stringToSign.String()))
	signature := hex.EncodeToString(h.Sum(nil))

	return c.JSON(fiber.Map{
		"signature": signature,
		"timestamp": timestamp,
		"apiKey":    os.Getenv("CLOUDINARY_API_KEY"),
		"cloudName": os.Getenv("CLOUDINARY_CLOUD_NAME"),
	})
}

func UploadMovieMedia(c *fiber.Ctx) error {
	movieId, ok := c.Locals("movieId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy MovieId",
		})
	}

	posterFile, ok := c.Locals("posterFile").(*multipart.FileHeader)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy file poster",
		})
	}

	trailerFile, ok := c.Locals("trailerFile").(*multipart.FileHeader)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy file trailer",
		})
	}

	isPrimary, ok := c.Locals("isPrimary").(bool)
	if !ok {
		isPrimary = false
	}

	cld, ok := c.Locals("cld").(*cloudinary.Cloudinary)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy Cloudinary client",
		})
	}

	// Mở file poster
	posterReader, err := posterFile.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể đọc file poster: %s", err.Error()),
		})
	}
	defer posterReader.Close()

	// Mở file trailer
	trailerReader, err := trailerFile.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể đọc file trailer: %s", err.Error()),
		})
	}
	var poster model.MoviePoster
	var trailer model.MovieTrailer
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// Nếu isPrimary = true, đặt các poster/trailer khác thành false
		if isPrimary {
			if err := tx.Model(&model.MoviePoster{}).Where("movie_id = ? AND is_primary = ?", movieId, true).Update("is_primary", false).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.MovieTrailer{}).Where("movie_id = ? AND is_primary = ?", movieId, true).Update("is_primary", false).Error; err != nil {
				return err
			}
		}

		// Tải poster lên Cloudinary
		posterResult, err := cld.Upload.Upload(context.Background(), posterReader, uploader.UploadParams{
			Folder:       "movies/posters",
			PublicID:     fmt.Sprintf("movie_%d_poster_%d", movieId, time.Now().Unix()),
			ResourceType: "image",
		})
		if err != nil {
			return fmt.Errorf("không thể tải poster lên Cloudinary: %v", err)
		}

		// Tạo MoviePoster
		poster = model.MoviePoster{
			MovieId:   movieId,
			Url:       &posterResult.SecureURL,
			IsPrimary: isPrimary,
		}
		if err := tx.Create(&poster).Error; err != nil {
			return err
		}

		// Tải trailer lên Cloudinary
		trailerResult, err := cld.Upload.Upload(context.Background(), trailerReader, uploader.UploadParams{
			Folder:       "movies/trailers",
			PublicID:     fmt.Sprintf("movie_%d_trailer_%d", movieId, time.Now().Unix()),
			ResourceType: "video",
		})
		if err != nil {
			return fmt.Errorf("không thể tải trailer lên Cloudinary: %v", err)
		}

		// Tạo MovieTrailer
		trailer = model.MovieTrailer{
			MovieId:   movieId,
			Url:       &trailerResult.SecureURL,
			IsPrimary: isPrimary,
		}
		if err := tx.Create(&trailer).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("%v", err),
		})
	}

	// Lấy lại bản ghi Movie
	var updatedMovie model.Movie
	if err := database.DB.Preload("Posters").Preload("Trailers").First(&updatedMovie, movieId).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể lấy thông tin phim: %s", err.Error()),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Upload media phim thành công",
		"data": fiber.Map{
			"movie":   updatedMovie,
			"poster":  poster,
			"trailer": trailer,
		},
	})
}
func UploadMultiplePosters(c *fiber.Ctx) error {
	movieId := c.Locals("movieId").(uint)
	files := c.Locals("posterFiles").([]*multipart.FileHeader)
	cld := c.Locals("cld").(*cloudinary.Cloudinary)
	primaryPosterId := c.Locals("primaryPosterId").(string) // Mới thêm: ID để set primary

	var createdPosters []model.MoviePoster
	var failedFiles []fiber.Map

	removePosterIds := c.Locals("removePosterIds").([]string)

	if len(removePosterIds) > 0 {
		for _, raw := range removePosterIds {
			id, err := strconv.Atoi(raw)
			if err != nil {
				continue
			}

			var poster model.MoviePoster
			if err := database.DB.First(&poster, id).Error; err == nil {
				// Xóa Cloudinary
				if poster.PublicID != nil {
					cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
						PublicID: *poster.PublicID,
					})
				}

				// Xóa trong DB
				database.DB.Delete(&poster)
			}
		}
	}

	for idx, file := range files {
		// Kiểm tra định dạng và kích thước (trước khi mở file)
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			failedFiles = append(failedFiles, fiber.Map{
				"filename": file.Filename,
				"error":    "Chỉ hỗ trợ JPG, PNG, WEBP",
			})
			continue
		}
		if file.Size > 5*1024*1024 {
			failedFiles = append(failedFiles, fiber.Map{
				"filename": file.Filename,
				"error":    "File vượt quá 5MB",
			})
			continue
		}

		// Mở file
		f, err := file.Open()
		if err != nil {
			failedFiles = append(failedFiles, fiber.Map{
				"filename": file.Filename,
				"error":    "Không thể mở file",
			})
			continue
		}

		// Upload lên Cloudinary
		publicID := fmt.Sprintf("movie_%d_poster_%d_%d", movieId, time.Now().UnixNano(), idx)
		uploadResult, err := cld.Upload.Upload(c.Context(), f, uploader.UploadParams{
			Folder:       "movies/posters",
			PublicID:     publicID,
			ResourceType: "image",
		})
		f.Close()

		if err != nil {
			failedFiles = append(failedFiles, fiber.Map{
				"filename": file.Filename,
				"error":    "Upload Cloudinary thất bại: " + err.Error(),
			})
			continue
		}

		// Tạm set isPrimary = false cho tất cả mới (sẽ xử lý primary sau)
		poster := model.MoviePoster{
			MovieId:   movieId,
			Url:       &uploadResult.SecureURL,
			IsPrimary: false,
			PublicID:  &uploadResult.PublicID,
		}

		if err := database.DB.Create(&poster).Error; err != nil {
			// Nếu lưu DB thất bại → xóa file trên Cloudinary
			cld.Upload.Destroy(context.Background(), uploader.DestroyParams{PublicID: uploadResult.PublicID})
			failedFiles = append(failedFiles, fiber.Map{
				"filename": file.Filename,
				"error":    "Lưu database thất bại",
			})
			continue
		}

		createdPosters = append(createdPosters, poster)
	}

	// Mới thêm: Xử lý set primary (cho cũ hoặc mới)
	if primaryPosterId != "" {
		if err := database.DB.Transaction(func(tx *gorm.DB) error {
			// Reset tất cả primary cũ về false
			if err := tx.Model(&model.MoviePoster{}).
				Where("movie_id = ? AND is_primary = ?", movieId, true).
				Update("is_primary", false).Error; err != nil {
				return err
			}

			// Set primary cho poster được chỉ định
			var targetPoster model.MoviePoster
			if primaryPosterId == "new_first" && len(createdPosters) > 0 {
				// Set cho poster mới đầu tiên
				targetPoster = createdPosters[0]
			} else {
				// Set cho poster cũ (ID số)
				id, err := strconv.Atoi(primaryPosterId)
				if err != nil {
					return errors.New("primaryPosterId không hợp lệ")
				}
				if err := tx.First(&targetPoster, id).Error; err != nil {
					return errors.New("poster không tồn tại")
				}
				if targetPoster.MovieId != movieId {
					return errors.New("poster không thuộc phim này")
				}
			}

			// Set primary = true
			if err := tx.Model(&targetPoster).Update("is_primary", true).Error; err != nil {
				return err
			}

			return nil
		}); err != nil {
			// Nếu lỗi, thêm vào response để thông báo
			failedFiles = append(failedFiles, fiber.Map{
				"error": "Không thể set primary poster: " + err.Error(),
			})
		}
	}

	// Preload lại movie với posters mới
	var updatedMovie model.Movie
	database.DB.Preload("Posters").Preload("Trailers").First(&updatedMovie, movieId)

	successCount := len(createdPosters)
	totalCount := len(files)

	response := fiber.Map{
		"message": fmt.Sprintf("Upload thành công %d/%d poster", successCount, totalCount),
		"data": fiber.Map{
			"movie":         updatedMovie,
			"uploaded":      createdPosters,
			"failed_files":  failedFiles,
			"success_count": successCount,
			"failed_count":  len(failedFiles),
		},
	}

	if successCount == 0 && len(failedFiles) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response)
	}

	return c.JSON(response)
}
func UploadTrailer(c *fiber.Ctx) error {
	movieId, ok := c.Locals("movieId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy MovieId",
		})
	}

	isPrimary, ok := c.Locals("isPrimary").(bool)
	if !ok {
		isPrimary = false
	}

	cld, ok := c.Locals("cld").(*cloudinary.Cloudinary)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy Cloudinary client",
		})
	}
	trailerFile, ok := c.Locals("trailerFile").(*multipart.FileHeader)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy file trailer",
		})
	}

	// Mở file trailer
	trailerReader, err := trailerFile.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể đọc file trailer: %s", err.Error()),
		})
	}
	var trailer model.MovieTrailer
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// Nếu isPrimary = true, đặt các poster/trailer khác thành false
		if isPrimary {
			if err := tx.Model(&model.MoviePoster{}).Where("movie_id = ? AND is_primary = ?", movieId, true).Update("is_primary", false).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.MovieTrailer{}).Where("movie_id = ? AND is_primary = ?", movieId, true).Update("is_primary", false).Error; err != nil {
				return err
			}
		}
		// Tải trailer lên Cloudinary
		trailerResult, err := cld.Upload.Upload(context.Background(), trailerReader, uploader.UploadParams{
			Folder:       "movies/trailers",
			PublicID:     fmt.Sprintf("movie_%d_trailer_%d", movieId, time.Now().Unix()),
			ResourceType: "video",
		})
		if err != nil {
			return fmt.Errorf("không thể tải trailer lên Cloudinary: %v", err)
		}

		// Tạo MovieTrailer
		trailer = model.MovieTrailer{
			MovieId:   movieId,
			Url:       &trailerResult.SecureURL,
			IsPrimary: isPrimary,
		}
		if err := tx.Create(&trailer).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("%v", err),
		})
	}

	// Lấy lại bản ghi Movie
	var updatedMovie model.Movie
	if err := database.DB.Preload("Posters").Preload("Trailers").First(&updatedMovie, movieId).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể lấy thông tin phim: %s", err.Error()),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Upload trailer phim thành công",
		"data": fiber.Map{
			"movie": updatedMovie,

			"trailer": trailer,
		},
	})
}

// handler.UploadMultipleTrailers
// func UploadMultipleTrailers(c *fiber.Ctx) error {
// 	movieId := c.Locals("movieId").(uint)
// 	files := c.Locals("trailerFiles").([]*multipart.FileHeader) // Sửa: slice multiple
// 	primaryTrailerId := c.Locals("primaryTrailerId").(string)
// 	cld := c.Locals("cld").(*cloudinary.Cloudinary)

// 	// Reset primary nếu cần
// 	var createdTrailers []model.MovieTrailer
// 	var failedFiles []fiber.Map

// 	removeTrailerIds := c.Locals("removeTrailerIds").([]string)

// 	// Xử lý xóa trailers cũ
// 	if len(removeTrailerIds) > 0 {
// 		for _, raw := range removeTrailerIds {
// 			id, err := strconv.Atoi(raw)
// 			if err != nil {
// 				continue
// 			}

// 			var trailer model.MovieTrailer
// 			if err := database.DB.First(&trailer, id).Error; err == nil {
// 				// Xóa Cloudinary nếu có PublicID (thêm nếu model có PublicID như poster)
// 				if trailer.PublicID != nil { // Giả sử model Trailer có PublicID pointer string
// 					cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
// 						PublicID: *trailer.PublicID,
// 					})
// 				}

// 				// Xóa trong DB
// 				database.DB.Delete(&trailer)
// 			}
// 		}
// 	}

// 	for idx, file := range files {
// 		// Kiểm tra định dạng và kích thước (ví dụ: mp4, < 100MB, tùy chỉnh)
// 		ext := strings.ToLower(filepath.Ext(file.Filename))
// 		if ext != ".mp4" { // Thêm các format khác nếu cần
// 			failedFiles = append(failedFiles, fiber.Map{
// 				"filename": file.Filename,
// 				"error":    "Chỉ hỗ trợ MP4",
// 			})
// 			continue
// 		}
// 		if file.Size > 100*1024*1024 { // Ví dụ 100MB
// 			failedFiles = append(failedFiles, fiber.Map{
// 				"filename": file.Filename,
// 				"error":    "File vượt quá 100MB",
// 			})
// 			continue
// 		}

// 		// Mở file
// 		src, err := file.Open()
// 		if err != nil {
// 			failedFiles = append(failedFiles, fiber.Map{"filename": file.Filename, "error": "Không thể mở file"})
// 			continue
// 		}

// 		buf := bufio.NewReaderSize(src, 4*1024*1024)
// 		// Upload lên Cloudinary
// 		publicID := fmt.Sprintf("movie_%d_trailer_%d_%d", movieId, time.Now().UnixNano(), idx)
// 		uploadResult, err := cld.Upload.Upload(
// 			context.Background(),
// 			buf,
// 			uploader.UploadParams{
// 				Folder:       "movies/trailers",
// 				PublicID:     publicID,
// 				ResourceType: "video",
// 			},
// 		)
// 		src.Close()
// 		if err != nil {
// 			failedFiles = append(failedFiles, fiber.Map{
// 				"filename": file.Filename,
// 				"error":    "Upload Cloudinary thất bại: " + err.Error(),
// 			})
// 			continue
// 		}

// 		// Tạm set isPrimary = false cho tất cả mới (sẽ xử lý primary sau)
// 		trailer := model.MovieTrailer{
// 			MovieId:   movieId,
// 			Url:       &uploadResult.SecureURL,
// 			IsPrimary: false,
// 			PublicID:  &uploadResult.PublicID, // Giả sử model có PublicID để xóa sau
// 		}

// 		if err := database.DB.Create(&trailer).Error; err != nil {
// 			// Nếu lưu DB thất bại → xóa file trên Cloudinary
// 			cld.Upload.Destroy(context.Background(), uploader.DestroyParams{PublicID: uploadResult.PublicID})
// 			failedFiles = append(failedFiles, fiber.Map{
// 				"filename": file.Filename,
// 				"error":    "Lưu database thất bại",
// 			})
// 			continue
// 		}

// 		createdTrailers = append(createdTrailers, trailer)
// 	}
// 	if primaryTrailerId != "" {
// 		if err := database.DB.Transaction(func(tx *gorm.DB) error {
// 			// Reset tất cả primary cũ về false (both poster & trailer)
// 			if err := tx.Model(&model.MoviePoster{}).
// 				Where("movie_id = ? AND is_primary = ?", movieId, true).
// 				Update("is_primary", false).Error; err != nil {
// 				return err
// 			}
// 			if err := tx.Model(&model.MovieTrailer{}).
// 				Where("movie_id = ? AND is_primary = ?", movieId, true).
// 				Update("is_primary", false).Error; err != nil {
// 				return err
// 			}

// 			// Set primary cho trailer được chỉ định
// 			var targetTrailer model.MovieTrailer
// 			if strings.HasPrefix(primaryTrailerId, "new_") && len(createdTrailers) > 0 {
// 				indexStr := strings.TrimPrefix(primaryTrailerId, "new_")
// 				index, err := strconv.Atoi(indexStr)
// 				if err != nil || index < 0 || index >= len(createdTrailers) {
// 					return errors.New("primaryTrailerId không hợp lệ cho trailer mới")
// 				}
// 				targetTrailer = createdTrailers[index]
// 			} else {
// 				// Set cho trailer cũ (ID số)
// 				id, err := strconv.Atoi(primaryTrailerId)
// 				if err != nil {
// 					return errors.New("primaryTrailerId không hợp lệ")
// 				}
// 				if err := tx.First(&targetTrailer, id).Error; err != nil {
// 					return errors.New("Trailer không tồn tại")
// 				}
// 				if targetTrailer.MovieId != movieId {
// 					return errors.New("Trailer không thuộc phim này")
// 				}
// 			}

// 			// Set primary = true
// 			if err := tx.Model(&targetTrailer).Update("is_primary", true).Error; err != nil {
// 				return err
// 			}

// 			return nil
// 		}); err != nil {
// 			// Nếu lỗi, thêm vào response
// 			failedFiles = append(failedFiles, fiber.Map{
// 				"error": "Không thể set primary trailer: " + err.Error(),
// 			})
// 		}
// 	}
// 	var updatedMovie model.Movie
// 	database.DB.Preload("Posters").Preload("Trailers").First(&updatedMovie, movieId)

// 	successCount := len(createdTrailers)
// 	totalCount := len(files)

// 	response := fiber.Map{
// 		"message": fmt.Sprintf("Upload thành công %d/%d trailer", successCount, totalCount),
// 		"data": fiber.Map{
// 			"movie":         updatedMovie,
// 			"uploaded":      createdTrailers,
// 			"failed_files":  failedFiles,
// 			"success_count": successCount,
// 			"failed_count":  len(failedFiles),
// 		},
// 	}

// 	if successCount == 0 && len(failedFiles) > 0 {
// 		return c.Status(fiber.StatusBadRequest).JSON(response)
// 	}

//		return c.JSON(response)
//	}
func UploadMultipleTrailers(c *fiber.Ctx) error {
	movieId := c.Locals("movieId").(uint)
	//primaryTrailerId := c.FormValue("primaryTrailerId") // Hoặc từ body nếu dùng JSON
	cld := c.Locals("cld").(*cloudinary.Cloudinary) // Vẫn cần để xóa nếu có

	type RequestBody struct {
		RemoveTrailerIds []int `json:"removeTrailerIds"`
		UploadedTrailers []struct {
			Url      string `json:"url"`
			PublicId string `json:"publicId"`
		} `json:"uploadedTrailers"`
		PrimaryTrailerId string `json:"primaryTrailerId"`
	}
	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Body không hợp lệ", err)
	}

	var createdTrailers []model.MovieTrailer
	var failedFiles []fiber.Map

	// Xử lý xóa trailers cũ (giữ nguyên)
	for _, id := range body.RemoveTrailerIds {
		var trailer model.MovieTrailer
		if err := database.DB.First(&trailer, id).Error; err == nil {
			if trailer.PublicID != nil {
				cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
					PublicID: *trailer.PublicID,
				})
			}
			database.DB.Delete(&trailer)
		}
	}

	// Lưu uploaded vào DB
	for idx, up := range body.UploadedTrailers {
		trailer := model.MovieTrailer{
			MovieId:   movieId,
			Url:       &up.Url,
			IsPrimary: false,
			PublicID:  &up.PublicId,
		}
		if err := database.DB.Create(&trailer).Error; err != nil {
			// Xóa Cloudinary nếu fail
			cld.Upload.Destroy(context.Background(), uploader.DestroyParams{PublicID: up.PublicId})
			failedFiles = append(failedFiles, fiber.Map{"index": idx, "error": "Lưu database thất bại"})
			continue
		}
		createdTrailers = append(createdTrailers, trailer)
	}

	// Set primary (giữ nguyên, dùng body.PrimaryTrailerId)
	if body.PrimaryTrailerId != "" {
		if err := database.DB.Transaction(func(tx *gorm.DB) error {
			// Reset primaries...
			if err := tx.Model(&model.MoviePoster{}).
				Where("movie_id = ? AND is_primary = ?", movieId, true).
				Update("is_primary", false).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.MovieTrailer{}).
				Where("movie_id = ? AND is_primary = ?", movieId, true).
				Update("is_primary", false).Error; err != nil {
				return err
			}
			var targetTrailer model.MovieTrailer
			if strings.HasPrefix(body.PrimaryTrailerId, "new_") && len(createdTrailers) > 0 {
				indexStr := strings.TrimPrefix(body.PrimaryTrailerId, "new_")
				index, err := strconv.Atoi(indexStr)
				if err != nil || index < 0 || index >= len(createdTrailers) {
					return errors.New("primaryTrailerId không hợp lệ cho trailer mới")
				}
				targetTrailer = createdTrailers[index]
			} else {
				id, err := strconv.Atoi(body.PrimaryTrailerId)
				if err != nil {
					return errors.New("primaryTrailerId không hợp lệ")
				}
				if err := tx.First(&targetTrailer, id).Error; err != nil {
					return errors.New("Trailer không tồn tại")
				}
				if targetTrailer.MovieId != movieId {
					return errors.New("Trailer không thuộc phim này")
				}
			}

			if err := tx.Model(&targetTrailer).Update("is_primary", true).Error; err != nil {
				return err
			}
			return nil
		}); err != nil {
			failedFiles = append(failedFiles, fiber.Map{"error": "Không thể set primary trailer: " + err.Error()})
		}
	}
	var updatedMovie model.Movie
	database.DB.Preload("Posters").Preload("Trailers").First(&updatedMovie, movieId)

	successCount := len(createdTrailers)
	totalCount := len(body.UploadedTrailers)

	response := fiber.Map{
		"message": fmt.Sprintf("Upload thành công %d/%d trailer", successCount, totalCount),
		"data": fiber.Map{
			"movie":         updatedMovie,
			"uploaded":      createdTrailers,
			"failed_files":  failedFiles,
			"success_count": successCount,
			"failed_count":  len(failedFiles),
		},
	}

	if successCount == 0 && len(failedFiles) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response)
	}

	return c.JSON(response)
}
