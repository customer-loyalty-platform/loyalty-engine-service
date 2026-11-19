package http

import (
	"loyalty-engine-service/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type StandardResponse = response.StandardResponse
type PaginationBody = response.PaginationBody
type ErrorBody = response.ErrorBody

func Success(c *gin.Context, httpStatus int, data interface{}) {
	response.Success(c, httpStatus, data)
}

func SuccessWithMeta(c *gin.Context, httpStatus int, data interface{}, pagination *PaginationBody) {
	response.SuccessWithMeta(c, httpStatus, data, pagination)
}

func Error(c *gin.Context, httpStatus int, code string, message string, details interface{}) {
	response.Error(c, httpStatus, code, message, details)
}
