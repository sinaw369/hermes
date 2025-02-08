package config

import "time"

type Config struct {
	GitlabBaseURL string
	GitlabToken   string
	SyncDir       string
	SyncInterval  time.Duration
}
