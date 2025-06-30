package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func main() {
	myuuid := uuid.New().String() // Generate a unique identifier

	for {
		now := time.Now().UTC()
		timestamp := now.Format("2006-01-02T15:04:05.000Z07:00") // ISO 8601 with milliseconds
		fmt.Printf("%s: %s\n", timestamp, myuuid)
		time.Sleep(5 * time.Second)
	}
}
