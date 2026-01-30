package main

import (
	"os"

	"intint64_db/internal/dbms"
)

const defaultPort = "7770"
const defaultSlots = 1024 * 1024

func main() {
	dataPath := envOr("DATA_PATH", "data.bin")
	metaPath := envOr("META_PATH", "meta_.bin")
	quantPath := envOr("QUANT_PATH", "quantize.bin")
	port := envOr("PORT", defaultPort)
	slots := int64(defaultSlots)
	if n := parseInt64(os.Getenv("SLOTS")); n > 0 {
		slots = n
	}

	store, err := dbms.OpenStore(dataPath, metaPath, quantPath, slots)
	if err != nil {
		os.Exit(1)
	}
	defer store.Close()

	listenAddr := ":" + port
	if err := dbms.RunServer(store, listenAddr); err != nil {
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	var n int64
	var sign int64 = 1
	for _, c := range s {
		if c == '-' {
			sign = -1
			continue
		}
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		}
	}
	return n * sign
}
