package main

import (
	"log"

	"github.com/LunaDeerTech/RsyncBackupService/internal/app"
	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	if err := app.New(cfg).Run(); err != nil {
		log.Fatal(err)
	}
}