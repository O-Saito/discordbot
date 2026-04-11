package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Token           string   `json:"token"`
	ApplicationID   string   `json:"applicationId"`
	VoiceChannelID  string   `json:"voiceChannelId"`
	GuildID         string   `json:"guildId"`
	MusicFolders    []string `json:"musicFolders"`
	RecursiveSearch bool     `json:"recursiveSearch"`
}

func NewConfig() *Config {
	configFile, err := os.Open("config.json")
	if err != nil {
		fmt.Println("Error opening config file:", err)
		return nil
	}
	defer configFile.Close()
	var config Config
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		fmt.Println("Error parsing config file:", err)
		return nil
	}
	return &config
}
