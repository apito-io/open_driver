package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/router"
	"github.com/apito-io/engine/utility"
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

	if err := refuseCloudData(&cfg); err != nil {
		log.Fatal(err)
	}

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

func refuseCloudData(cfg *models.Config) error {
	if cfg.ServePort == "5050" {
		return fmt.Errorf("public engine must not listen on 5050; that port belongs to the Cloud engine")
	}
	checks := []string{cfg.SQLiteDataDir, cfg.DefaultDatabaseDir, cfg.PluginPath, cfg.PrivateKeyPath, cfg.PublicKeyPath}
	for _, raw := range checks {
		expanded, err := utility.ExpandPath(raw)
		if err != nil {
			return fmt.Errorf("expand %s: %w", raw, err)
		}
		cleaned := filepath.Clean(expanded)
		if strings.Contains(cleaned, string(filepath.Separator)+"engine"+string(filepath.Separator)+"db") ||
			strings.Contains(cleaned, string(filepath.Separator)+"engine"+string(filepath.Separator)+"plugins") ||
			strings.Contains(cleaned, string(filepath.Separator)+"engine"+string(filepath.Separator)+"keys") ||
			strings.Contains(cleaned, string(filepath.Separator)+".apito"+string(filepath.Separator)+"db") {
			return fmt.Errorf("public engine must not use Cloud data at %s", cleaned)
		}
	}
	return nil
}
