package config

import (
	"fmt"
	"github.com/spf13/viper"
	"path/filepath"
	"time"
)

func loadString(envName string) string {
	validate(envName)

	return viper.GetString(envName)
}
func loadFilePath(envName string) string {
	validate(envName)
	path := viper.GetString(envName)

	if !filepath.IsAbs(path) {
		panic(fmt.Sprintf("[%s] should be full path : %s", envName, path))
	}
	return path
}
func loadInt(envName string) int {
	validate(envName)

	return viper.GetInt(envName)
}
func LoadDuration(envName string) time.Duration {
	validate(envName)

	return viper.GetDuration(envName)
}

func loadBool(envName string) bool {
	validate(envName)

	return viper.GetBool(envName)
}
func loadFloat64(envName string) float64 {
	validate(envName)

	return viper.GetFloat64(envName)

}

func loadStringSlice(envName string) []string {
	validate(envName)

	return viper.GetStringSlice(envName)
}

func validate(envName string) {
	exists := viper.IsSet(envName)
	if !exists {
		panic(fmt.Sprintf("environment variable [%s] does not exist", envName))
	}
}
