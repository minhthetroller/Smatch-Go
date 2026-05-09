package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port    int
	NodeEnv string

	DatabaseURL    string
	TileServerURL  string
	TileLayerID    string
	SlotLockTTLSec int

	Redis struct {
		Host       string
		Port       int
		Password   string
		TLSEnabled bool
	}

	ZaloPay struct {
		AppID       string
		Key1        string
		Key2        string
		Endpoint    string
		CallbackURL string
	}

	// FirebaseCredentialsFile is the path to the Admin SDK service-account JSON
	// downloaded from the Firebase console.
	FirebaseCredentialsFile string

	Blob struct {
		AccountName          string
		AccountKey           string
		Endpoint             string // optional: Azurite override
		ContainerProfile     string
		ContainerMatches     string
		ContainerBusinessDocs string
	}

	AdminSecret   string
	AdminPort     int
	AdminWebOrigin string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("[config] .env not loaded: %v\n", err)
	} else {
		fmt.Println("[config] .env loaded")
	}

	cfg := &Config{}

	cfg.Port = getEnvInt("PORT", 3000)
	cfg.NodeEnv = getEnv("NODE_ENV", "development")
	cfg.DatabaseURL = getEnv("DATABASE_URL", "")
	cfg.TileServerURL = getEnv("TILE_SERVER_URL", "http://localhost:7800")
	cfg.TileLayerID = getEnv("TILE_LAYER_ID", "public.courts")
	cfg.SlotLockTTLSec = getEnvInt("SLOT_LOCK_TTL_SECONDS", 600)

	cfg.Redis.Host = getEnv("REDIS_HOST", "localhost")
	cfg.Redis.Port = getEnvInt("REDIS_PORT", 6379)
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
	cfg.Redis.TLSEnabled = getEnv("REDIS_TLS_ENABLED", "false") == "true"

	cfg.ZaloPay.AppID = getEnv("ZALOPAY_APP_ID", "")
	cfg.ZaloPay.Key1 = getEnv("ZALOPAY_KEY1", "")
	cfg.ZaloPay.Key2 = getEnv("ZALOPAY_KEY2", "")
	cfg.ZaloPay.Endpoint = getEnv("ZALOPAY_ENDPOINT", "https://sb-openapi.zalopay.vn")
	cfg.ZaloPay.CallbackURL = getEnv("ZALOPAY_CALLBACK_URL", "")

	cfg.FirebaseCredentialsFile = getEnv("FIREBASE_CREDENTIALS_FILE", "smatch-badminton-firebase-adminsdk-fbsvc-fb65abab30.json")

	cfg.Blob.AccountName = getEnv("AZURE_STORAGE_ACCOUNT", "")
	cfg.Blob.AccountKey = getEnv("AZURE_STORAGE_KEY", "")
	cfg.Blob.Endpoint = getEnv("AZURE_BLOB_ENDPOINT", "")
	cfg.Blob.ContainerProfile = getEnv("AZURE_STORAGE_CONTAINER_PROFILE", "smatch-profiles")
	cfg.Blob.ContainerMatches = getEnv("AZURE_STORAGE_CONTAINER_MATCHES", "smatch-matches")
	cfg.Blob.ContainerBusinessDocs = getEnv("AZURE_STORAGE_CONTAINER_BUSINESS_DOCS", "smatch-business-docs")

	cfg.AdminSecret = getEnv("ADMIN_SECRET", "")
	cfg.AdminPort = getEnvInt("ADMIN_PORT", 3001)
	cfg.AdminWebOrigin = getEnv("ADMIN_WEB_ORIGIN", "https://admin-sb.online")

	return cfg
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
