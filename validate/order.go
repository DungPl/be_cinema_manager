package validate

// func CreateOrderValidate() fiber.Handler {
// 	return func(c *fiber.Ctx) error {
// 		var input model.CreateOrderInput
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

// 		customerId, _ := helper.GetInfoCustomerFromToken(c)
// 		tx := database.DB.Begin()

// 		// 1. CHECK RESERVATION ACTIVE
// 		var reservation model.Reservation
// 		tx.Preload("Showtime").Where("id = ? AND customer_id = ? AND status = ?",
// 			input.ReservationId, customerId, "ACTIVE").First(&reservation)

// 		// 2. CHECK NOT EXPIRED
// 		if time.Now().After(reservation.ExpiresAt) {
// 			tx.Rollback()
// 			return c.Status(400).JSON(fiber.Map{"error": "Reservation expired"})
// 		}

// 		// 3. CHECK NOT HAVE ORDER YET
// 		var existingOrder model.Order
// 		tx.Where("reservation_id = ?", input.ReservationId).First(&existingOrder)
// 		if existingOrder.ID > 0 {
// 			tx.Rollback()
// 			return c.Status(400).JSON(fiber.Map{"error": "Order already exists"})
// 		}

// 		// 4. CALCULATE TOTAL (Multiple seats)
// 		totalAmount := float64(0)
// 		for _, seat := range reservation.Seats {
// 			totalAmount += float64(reservation.Showtime.Price) * seat.SeatType.PriceModifier
// 		}

// 		c.Locals("input", input)
// 		c.Locals("customerId", customerId)
// 		c.Locals("totalAmount", totalAmount)
// 		c.Locals("tx", tx)
// 		return c.Next()
// 	}
// }
