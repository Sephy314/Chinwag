package router

import (
	"io/fs"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/chat/command/docs"
	"github.com/labstack/echo/v5"
)

func SetUpSwaggerRoutes(e *echo.Echo) {
	e.GET("/swagger/swagger.json", func(c *echo.Context) error {
		return c.JSONBlob(http.StatusOK, docs.SwaggerJSON)
	})

	staticFS, _ := fs.Sub(docs.StaticFS, "swagger-ui")
	fileServer := http.StripPrefix("/swagger/swagger-ui/", http.FileServer(http.FS(staticFS)))

	e.GET("/swagger/swagger-ui/*", func(c *echo.Context) error {
		path := c.Request().URL.Path
		if path == "/swagger/swagger-ui/" || path == "/swagger/swagger-ui" {
			http.Redirect(c.Response(), c.Request(), "/swagger", http.StatusMovedPermanently)
			return nil
		}
		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	e.GET("/swagger", func(c *echo.Context) error {
		return c.HTML(http.StatusOK, string(docs.IndexHTML))
	})
}
