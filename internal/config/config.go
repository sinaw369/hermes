package config

import "time"

type Config struct {
	GitlabBaseURL  string
	GitlabToken    string
	SyncDir        string
	FileDir        string
	DiffBranchFrom string
	DifBranchTO    string
	SyncInterval   time.Duration
}
