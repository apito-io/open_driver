package main

import (
	"fmt"
	"log"
	"os"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/router"
	odfactory "github.com/apito-io/open_driver/factory"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Overload(".env"); err != nil {
		fmt.Printf("Info: No .env file found or error loading it: %v\n", err)
	}

	var cfg models.Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("config: %v", err)
	}

	cfg.DriverFactory = odfactory.NewDefaultFactory()
	cfg.CacheFactory = odfactory.NewDefaultCacheFactory()
	cfg.KVFactory = odfactory.NewDefaultKVFactory()
	// In-process realtime. A functions-enabled host can inject a networked bus.
	cfg.RealtimeBusFactory = odfactory.NewDefaultMemoryRealtimeFactory()
	// Plugins are optional. This host boots without a plugin supervisor.
	cfg.PluginHost = odfactory.NewNoopPluginHost()

	_route, err := router.InitRouter(&cfg)
	if err != nil {
		ae.EwP(err, "Router Init")
		log.Fatal(err)
	}

	port := cfg.ServePort
	if port == "" {
		port = "5051"
	}
	log.Printf("Apito engine listening on :%s", port)
	if err := _route.Start(":" + port); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
