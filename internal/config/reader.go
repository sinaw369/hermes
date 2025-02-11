package config

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
)

func Load() (config *Config, err error) {
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.SetConfigName(".env")
	viper.AllowEmptyEnv(true)
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if !errors.As(err, &viper.ConfigFileNotFoundError{}) {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	return &Config{
		GitlabBaseURL:  loadString("GITLAB_BASE_URL"),
		GitlabToken:    loadString("GITLAB_TOKEN"),
		SyncDir:        loadString("SYNC_DIR"),
		FileDir:        loadString("FILES_DIR"),
		DiffBranchFrom: loadString("DIFF_BRANCH_FROM"),
		DifBranchTO:    loadString("DIFF_BRANCH_TO"),
		SyncInterval:   LoadDuration("SYNC_INTERVAL"),
	}, nil

}
