package main

import (
	"encoding/json"
	"fmt"
	"os"

	"mydiscordbot/bot"
)

func main() {
	configFile, err := os.Open("config.json")
	if err != nil {
		fmt.Println("Error opening config file:", err)
		return
	}
	defer configFile.Close()

	var config bot.Config
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		fmt.Println("Error parsing config file:", err)
		return
	}

	manager, err := bot.NewManager(config.Token, config.VoiceChannelID, config.GuildID, config.MusicFolders, config.RecursiveSearch)
	if err != nil {
		fmt.Println("Error creating bot manager:", err)
		return
	}

	if err := manager.Start(); err != nil {
		fmt.Println("Error starting bot:", err)
		return
	}

	manager.WaitForSignal()

	if err := manager.Stop(); err != nil {
		fmt.Println("Error stopping bot:", err)
	}
}
