package main

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

var (
	// COMMIT_SHA and COMMIT_TAG are set by the build system
	COMMIT_SHA string
	COMMIT_TAG string
)

func main() {
	myuuid := uuid.New().String()

	arglen := len(os.Args[1:])

	if arglen == 0 {
		fmt.Printf("Usage: %s <filename>\n", os.Args[0])
		os.Exit(1)
	} else if arglen > 1 {
		fmt.Printf("Please give only one filename. Usage: %s <filename>\n", os.Args[0])
		os.Exit(1)
	}

	fname := os.Args[1]

	fmt.Printf("Starting app1 (SHA %s) with UUID: %s\n", COMMIT_SHA, myuuid)

	for {
		func() {
			fp, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

			if err != nil {
				fmt.Printf("Error opening log file: %v\n", err)
			}
			defer fp.Close()

			logLine := logString(myuuid)
			_, err = fp.WriteString(logLine + "\n")
			if err != nil {
				fmt.Printf("Error writing to log file: %v\n", err)
			}

			time.Sleep(5 * time.Second)
		}()
	}

}

// logString returns the formatted log string
func logString(id string) string {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
	return fmt.Sprintf("%s: %s", timestamp, id)
}
