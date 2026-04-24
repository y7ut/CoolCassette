package api

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// writeError maps common application errors to HTTP responses.
func writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	type conflictWithMetadata interface {
		ConflictMetadata() IndexSnapshot
	}
	var conflictErr conflictWithMetadata
	switch {
	case errors.Is(err, os.ErrNotExist):
		status = http.StatusNotFound
	case errors.Is(err, context.Canceled):
		status = 499
	case errors.As(err, &conflictErr):
		status = http.StatusConflict
		writeIndexHeaders(c, conflictErr.ConflictMetadata())
		c.JSON(status, gin.H{
			"error":           err.Error(),
			"index_version":   conflictErr.ConflictMetadata().Version,
			"index_hash":      conflictErr.ConflictMetadata().Hash,
			"reload_required": true,
		})
		return
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

// writeIndexHeaders writes the active index version/hash headers onto a response.
func writeIndexHeaders(c *gin.Context, snapshot IndexSnapshot) {
	if snapshot.Version != "" {
		c.Header(HeaderIndexVersion, snapshot.Version)
	}
	if snapshot.Hash != "" {
		c.Header(HeaderIndexHash, snapshot.Hash)
	}
}
