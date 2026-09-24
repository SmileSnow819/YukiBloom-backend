package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Config struct {
	Port         string
	DatabaseURL  string
	CookieSecure bool
}

func Load(getenv func(string) string) (Config, error) {
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL 不能为空")
	}

	port := strings.TrimSpace(getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return Config{}, fmt.Errorf("PORT 必须是 1 到 65535 之间的数字")
	}

	cookieSecure := true
	if raw := strings.TrimSpace(getenv("COOKIE_SECURE")); raw != "" {
		cookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE 必须是 true 或 false")
		}
	}

	return Config{Port: port, DatabaseURL: databaseURL, CookieSecure: cookieSecure}, nil
}
