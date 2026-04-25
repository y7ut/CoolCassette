package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *LibraryHandler) Reload(c *gin.Context) {
	var req ReloadRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		writeError(c, err)
		return
	}
	value, err := h.service.ReloadLibrary(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.JSON(http.StatusAccepted, value)
}
