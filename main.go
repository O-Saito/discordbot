package main

import (
	"fmt"

	"mydiscordbot/bot"
	"mydiscordbot/commands"
	"mydiscordbot/config"
)

func main() {

	bot.RegisterCommand(&commands.AddCommand{})
	bot.RegisterCommand(&commands.MusicCommand{})
	bot.RegisterCommand(&commands.JoinCommand{})

	manager, err := bot.NewManager(config.NewConfig())
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
