package main

import (
	"fmt"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
)

func main() {
	cfg := config.LoadWorker()

	fmt.Printf("GophProfile worker started\n")
	fmt.Printf("log level: %s\n", cfg.LogLevel)
}
