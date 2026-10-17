package bazarr

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	BaseURL string
	APIKey  string
	Listen  string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		BaseURL: strings.TrimRight(os.Getenv("BAZARR_URL"), "/"),
		APIKey:  os.Getenv("BAZARR_API_KEY"),
		Listen:  os.Getenv("BAZARR_LISTEN_ADDR"),
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8080"
	}
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateConfig(cfg Config) error {
	if cfg.BaseURL == "" {
		return fmt.Errorf("BAZARR_URL is required")
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("BAZARR_URL must be an http(s) URL without userinfo, query, or fragment")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("BAZARR_API_KEY is required")
	}
	return nil
}
