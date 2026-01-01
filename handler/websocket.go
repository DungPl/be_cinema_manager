package handler

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
)

var (
	redisClient = redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	clients = make(map[uint]map[*websocket.Conn]bool)
	mu      sync.Mutex
)

// WebSocketConnection xử lý WS connection
func WebSocketConnection(c *websocket.Conn) {
	// Lấy showtimeId từ route
	showtimeIdStr := c.Params("id")
	id64, _ := strconv.ParseUint(showtimeIdStr, 10, 64)
	showtimeId := uint(id64)

	// Khi WS disconnect → xoá client
	defer func() {
		mu.Lock()
		if clients[showtimeId] != nil {
			delete(clients[showtimeId], c)
		}
		mu.Unlock()
		c.Close()
	}()

	// Thêm client mới vào room
	mu.Lock()
	if clients[showtimeId] == nil {
		clients[showtimeId] = make(map[*websocket.Conn]bool)
	}
	clients[showtimeId][c] = true
	mu.Unlock()

	// Gửi danh sách ghế lần đầu
	seats, _ := FetchShowtimeSeats(showtimeId)
	c.WriteJSON(seats)

	// Sub kênh Redis
	pubsub := redisClient.Subscribe(
		context.Background(),
		fmt.Sprintf("showtime:%d", showtimeId),
	)
	defer pubsub.Close()

	// Lắng nghe message từ Redis
	channel := pubsub.Channel()

	for msg := range channel {
		payload := []byte(msg.Payload)

		mu.Lock()
		for conn := range clients[showtimeId] {
			// Nếu client lỗi → xoá
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				conn.Close()
				delete(clients[showtimeId], conn)
			}
		}
		mu.Unlock()
	}
}
