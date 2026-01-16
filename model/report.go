package model

type DashboardKPI struct {
	TotalRevenue    float64 `json:"totalRevenue"`
	TicketsSold     int64   `json:"ticketsSold"`
	OccupancyRate   float64 `json:"occupancyRate"`
	UniqueCustomers int64   `json:"uniqueCustomers"`
	RevenueChange   float64 `json:"revenueChangePct"` // % thay đổi so với kỳ trước
}

type TopMovieItem struct {
	Title          string  `json:"title"`
	Revenue        float64 `json:"revenue"`
	Tickets        int64   `json:"tickets"`
	ShowtimesCount int64   `json:"showtimesCount"`
	OccupancyAvg   float64 `json:"occupancyAvg"`
	AvgRating      float64 `json:"avgRating"` // Nếu Movie có rating
}

type RevenueByCinemaItem struct {
	CinemaName     string  `json:"cinemaName"`
	Revenue        float64 `json:"revenue"`
	Tickets        int64   `json:"tickets"`
	Occupancy      float64 `json:"occupancy"`
	AvgTicketPrice float64 `json:"avgTicketPrice"`
}

type OccupancyTrendItem struct {
	Date string  `json:"date"` // Format: 02/01
	Rate float64 `json:"rate"`
}

type DashboardSummary struct {
	TotalRevenue     float64 `json:"totalRevenue"`
	TotalTickets     int64   `json:"totalTickets"`
	AvgOccupancy     float64 `json:"avgOccupancy"`
	TotalCustomers   int64   `json:"totalCustomers"`
	RevenueChangePct float64 `json:"revenueChangePct"`
}

type DashboardReport struct {
	Items          []interface{}         `json:"items"` // Linh hoạt cho top movies/revenue
	Summary        *DashboardSummary     `json:"summary"`
	Pagination     *PaginationInfo       `json:"pagination,omitempty"`
	Trends         []OccupancyTrendItem  `json:"trends,omitempty"`
	TopMovies      []TopMovieItem        `json:"top_movies"`
	RevenueCinemas []RevenueByCinemaItem `json:"revenue_cinemas"`
}

type PaginationInfo struct {
	CurrentPage int  `json:"currentPage"`
	TotalPages  int  `json:"totalPages"`
	TotalItems  int  `json:"totalItems"`
	Limit       int  `json:"limit"`
	HasNext     bool `json:"hasNext"`
	HasPrev     bool `json:"hasPrev"`
}
