// @title Chinwag API
// @version 1.0
// @description Chinwag Chat Application API
// @host localhost:8000
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"log"

	"github.com/Sephy314/chinwag/router"
)

func main() {
	e, err := router.SetUpRouter()

	if err != nil {
		log.Fatal(err)
	}
	err = e.Start("0.0.0.0:8000")
	if err != nil {
		log.Fatal(err)
	}
}
