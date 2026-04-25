package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *LibraryHandler) ClearCache(c *gin.Context) {
	value, err := h.service.ClearCache(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}
