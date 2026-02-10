package models

// ApiResponse is the standard API response envelope
type ApiResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// PaginatedResponse extends ApiResponse with pagination info
type PaginatedResponse[T any] struct {
	Success    bool        `json:"success"`
	Data       T           `json:"data,omitempty"`
	Error      string      `json:"error,omitempty"`
	Message    string      `json:"message,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination holds pagination metadata
type Pagination struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"totalPages"`
}

// NewSuccessResponse creates a successful API response
func NewSuccessResponse[T any](data T) ApiResponse[T] {
	return ApiResponse[T]{
		Success: true,
		Data:    data,
	}
}

// NewSuccessResponseWithMessage creates a successful API response with a message
func NewSuccessResponseWithMessage[T any](data T, message string) ApiResponse[T] {
	return ApiResponse[T]{
		Success: true,
		Data:    data,
		Message: message,
	}
}

// NewErrorResponse creates an error API response
func NewErrorResponse(err string) ApiResponse[any] {
	return ApiResponse[any]{
		Success: false,
		Error:   err,
	}
}

// NewPaginatedResponse creates a paginated API response
func NewPaginatedResponse[T any](data T, pagination *Pagination) PaginatedResponse[T] {
	return PaginatedResponse[T]{
		Success:    true,
		Data:       data,
		Pagination: pagination,
	}
}
