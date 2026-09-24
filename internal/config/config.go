package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func Load(getenv func(string) string) (Config, error) {
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	port := strings.TrimSpace(getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return Config{}, fmt.Errorf("PORT must be a number from 1 to 65535")
	}

	return Config{Port: port, DatabaseURL: databaseURL}, nil
}
