package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
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

func Setup() Config {

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

func SetupConfig() Config {
	var apiKey string
	BaseUrl := "https://api.xiaomimimo.com/v1"
	Model := "xiaomi/mimo-v2.5"
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println("Enter your ApiKey: ")
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
			if apiKey != "" {
				break
			}
			fmt.Println("Please enter a valid APIKEY")
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	key := fmt.Sprintf("APIKEY=%s", apiKey)

	fmt.Println("Enter BaseURL [default: https://api.xiaomimimo.com/v1]: ")
	fmt.Scan(&BaseUrl)

	fmt.Println("Enter Model [default: xiaomi/mimo-v2.5]: ")
	fmt.Scan(&Model)

	return Config{Model: Model, BaseUrl: BaseUrl, ApiKey: key}
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
	newPath := filepath.Join(path, "config.toml")
	file, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)

	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	return toml.NewEncoder(file).Encode(data)
}

func readConfig(path string) (Config, error) {
	var cfg Config
	newPath := filepath.Join(path, "config.toml")
	toml.DecodeFile(newPath, &cfg)
	return cfg, nil
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
