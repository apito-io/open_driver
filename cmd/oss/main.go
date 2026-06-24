package main

import (
	"fmt"
	"log"
	"os"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/router"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
	"gitlab.com/apito.io/nats_system"
	odfactory "gitlab.com/apito.io/open_driver/factory"
	pluginhost "gitlab.com/apito.io/plugin_system/hashicorp"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		fmt.Printf("Info: No .env file found or error loading it: %v\n", err)
	}

	var cfg models.Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("config: %v", err)
	}

	cfg.DriverFactory = odfactory.NewDefaultFactory()
	cfg.CacheFactory = odfactory.NewDefaultCacheFactory()
	cfg.KVFactory = odfactory.NewDefaultKVFactory()
	cfg.RealtimeBusFactory = nats_system.NewFactory()
	cfg.PluginHost = pluginhost.NewHost()

	_route, err := router.InitRouter(&cfg)
	if err != nil {
		ae.EwP(err, "Router Init")
		log.Fatal(err)
	}

	port := cfg.ServePort
	if port == "" {
		port = "5050"
	}
	log.Printf("Apito open-core listening on :%s", port)
	if err := _route.Start(":" + port); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
