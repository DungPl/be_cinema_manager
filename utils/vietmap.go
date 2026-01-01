package utils

import (
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/gofiber/fiber/v2"
)

func SetupVietmapRoutes(app *fiber.App) {
	// === AUTOCOMPLETE (tìm kiếm gợi ý địa chỉ) ===
	app.Get("/api/vietmap/autocomplete", func(c *fiber.Ctx) error {
		query := c.Query("text")
		if query == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "text parameter required"})
		}

		apiKey := os.Getenv("VIETMAP_API_KEY")
		if apiKey == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "VIETMAP_API_KEY not set"})
		}

		// Dùng v3 để display sạch và có ref_id
		vietmapURL := "https://maps.vietmap.vn/api/autocomplete/v3?apikey=" + apiKey + "&text=" + url.QueryEscape(query)

		resp, err := http.Get(vietmapURL)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "cannot reach Vietmap autocomplete"})
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)

		c.Status(resp.StatusCode)
		c.Set("Content-Type", "application/json")
		return c.Send(bodyBytes)
	})

	// === PLACE DETAIL (lấy tọa độ chính xác từ ref_id) ===
	app.Get("/api/vietmap/place", func(c *fiber.Ctx) error {
		refID := c.Query("refid")
		if refID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "refid parameter required"})
		}

		apiKey := os.Getenv("VIETMAP_API_KEY")

		placeURL := "https://maps.vietmap.vn/api/place/v3?apikey=" + apiKey + "&refid=" + url.QueryEscape(refID)

		resp, err := http.Get(placeURL)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "cannot reach Vietmap place"})
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)

		c.Status(resp.StatusCode)
		c.Set("Content-Type", "application/json")
		return c.Send(bodyBytes)
	})
}
