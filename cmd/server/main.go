package main

import (
	"fmt"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
)

func main() {
	cfg := config.LoadServer()

	fmt.Printf("GophProfile server started\n")
	fmt.Printf("address: %s\n", cfg.Address)
	fmt.Printf("log level: %s\n", cfg.LogLevel)
}
