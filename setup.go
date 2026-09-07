package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	// "strings"
)

type Config struct {
	Model   string `toml:"model"`
	BaseUrl string `toml:"base_url"`
	ApiKey  string `toml:"api_key"`
}

func Setup() {

	path, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	cfgPath := filepath.Join(path, "config.toml")

	if !FileExists(cfgPath) {
		cfg := SetupConfig()
		writeConfig(cfgPath, cfg)
	}
}

func CheckConfig(path string) Config {
	cfg, err := loadConfig(path)
	if err != nil {
		log.Fatal(err)
	}

	if cfg.BaseUrl == "" {
		cfg.BaseUrl = "https://api.xiaomimimo.com/v1"
	}

	if cfg.Model == "" {
		cfg.Model = "xiaomi/mimo-v2.5"
	}

	return cfg
}

func CLI() {
	path, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	cfgPath := filepath.Join(path, "config.toml")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			OpenChosenFile(cfgPath)
		case "setup":
			if FileExists(cfgPath) {
				OpenChosenFile(cfgPath)
			} else {
				data := SetupConfig()
				writeConfig(path, data)
			}
		}
	}

	os.Exit(0)
}

func SetupConfig() Config {

	scanner := bufio.NewScanner(os.Stdin)

	default_url := "https://api.xiaomimimo.com/v1"
	default_model := "xiaomi/mimo-v2.5"

	key := readLine(scanner, "Enter ApiKey: ")
	url := readLine(scanner, "Enter BaseURL [default: https://api.xiaomimimo.com/v1]: ")
	model := readLine(scanner, "Enter Model [xiaomi/mimo-v2.5]: ")

	if url == "" {
		url = default_url
	}

	if model == "" {
		model = default_model
	}

	return Config{Model: model, BaseUrl: url, ApiKey: key}
}

func readLine(scanner *bufio.Scanner, prompt string) string {

	fmt.Print(prompt)

	for scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return ""
}

func FileExists(path string) bool {
	_, err := os.Stat(path)

	if err == nil {
		return true
	}

	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	return false
}
func getConfig() (string, error) {
	config, err := os.UserConfigDir()

	if err != nil {
		log.Fatal(err)
	}

	appDir := filepath.Join(config, "Lucky")

	err = os.MkdirAll(appDir, 0755)

	if err != nil {
		log.Fatal(err)
	}

	return appDir, nil
}

func writeConfig(path string, data Config) error {
	file, err := os.Create(path)

	if err != nil {
		panic(fmt.Sprintf("Error creating file %v", err))
	}

	defer file.Close()

	return toml.NewEncoder(file).Encode(data)
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	newPath := filepath.Join(path, "config.toml")
	file, err := os.Open(newPath)
	if err != nil {
		return cfg, err
	}

	defer file.Close()

	err = toml.NewDecoder(file).Decode(&cfg)
	return cfg, err
}

func OpenChosenFile(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", path)
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Run()

}
