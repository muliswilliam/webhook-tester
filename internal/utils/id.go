package utils

import (
	"log"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

func GenerateID() string {
	id, err := gonanoid.New()
	if err != nil {
		log.Print("error creating id: ", err)
	}
	return id
}
