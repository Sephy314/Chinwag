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
	err = e.Start("0.0.0.0:10080")
	if err != nil {
		log.Fatal(err)
	}
}
