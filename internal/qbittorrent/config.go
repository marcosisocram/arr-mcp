package qbittorrent

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	BaseURL  string
	APIKey   string
	Username string
	Password string
	Listen   string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		BaseURL:  strings.TrimRight(os.Getenv("QBITTORRENT_URL"), "/"),
		APIKey:   os.Getenv("QBITTORRENT_API_KEY"),
		Username: os.Getenv("QBITTORRENT_USERNAME"),
		Password: os.Getenv("QBITTORRENT_PASSWORD"),
		Listen:   os.Getenv("QBITTORRENT_LISTEN_ADDR"),
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
		return fmt.Errorf("QBITTORRENT_URL is required")
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("QBITTORRENT_URL must be an http(s) URL without userinfo, query, or fragment")
	}
	if cfg.APIKey == "" && (cfg.Username == "" || cfg.Password == "") {
		return fmt.Errorf("set QBITTORRENT_API_KEY or both QBITTORRENT_USERNAME and QBITTORRENT_PASSWORD")
	}
	return nil
}
