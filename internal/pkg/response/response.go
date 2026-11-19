package response

import (
	"time"

	"github.com/gin-gonic/gin"
)

type StandardResponse struct {
	Status     string          `json:"status"`
	Timestamp  string          `json:"timestamp"`
	Data       interface{}     `json:"data,omitempty"`
	Pagination *PaginationBody `json:"pagination,omitempty"`
	Error      *ErrorBody      `json:"error,omitempty"`
}

type PaginationBody struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func Success(c *gin.Context, httpStatus int, data interface{}) {
	c.JSON(httpStatus, StandardResponse{
		Status:    "SUCCESS",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	})
}

func SuccessWithMeta(c *gin.Context, httpStatus int, data interface{}, pagination *PaginationBody) {
	c.JSON(httpStatus, StandardResponse{
		Status:     "SUCCESS",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Data:       data,
		Pagination: pagination,
	})
}

func Error(c *gin.Context, httpStatus int, code string, message string, details interface{}) {
	c.JSON(httpStatus, StandardResponse{
		Status:    "ERROR",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Error: &ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
