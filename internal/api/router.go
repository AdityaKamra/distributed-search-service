package api

import "github.com/gin-gonic/gin"

func SetupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/health", h.HealthCheck)

	v1 := r.Group("/")
	v1.Use(h.RateLimitMiddleware())
	{
		v1.POST("/documents", h.IndexDocument)
		v1.GET("/search", h.SearchDocuments)
		v1.GET("/documents/:id", h.GetDocument)
		v1.DELETE("/documents/:id", h.DeleteDocument)
	}

	return r
}
