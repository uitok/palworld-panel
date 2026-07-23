package api

import "github.com/gin-gonic/gin"

func (s Server) breedingStatus(c *gin.Context) {
	ok(c, s.breeding.Status(c.Request.Context()))
}
