package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DifyBaseURL    string
	DifyAPIKey     string
	DifyProxyURL   string
	ListenAddr     string
	DefaultUser    string
	RequestTimeout time.Duration
	// A2A
	A2AEnabled bool
	A2APort    int
	AgentName  string
	AgentDesc  string
}

func Load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.DifyBaseURL, "dify-base-url", getEnv("DIFY_BASE_URL", "http://localhost"), "Dify instance base URL or full endpoint URL")
	flag.StringVar(&cfg.DifyAPIKey, "dify-api-key", getEnv("DIFY_API_KEY", ""), "Dify API key (required for A2A; proxy passes caller's key)")
	flag.StringVar(&cfg.DifyProxyURL, "dify-proxy-url", getEnv("DIFY_PROXY_URL", ""), "HTTP/HTTPS proxy URL for Dify requests (e.g. http://proxy:8080)")
	flag.StringVar(&cfg.ListenAddr, "listen-addr", getEnv("LISTEN_ADDR", ":8080"), "Proxy listen address")
	flag.StringVar(&cfg.DefaultUser, "default-user", getEnv("DEFAULT_USER", "dify-agent"), "Default user field for Dify requests")

	timeoutStr := getEnv("REQUEST_TIMEOUT", "120s")
	defaultTimeout, _ := time.ParseDuration(timeoutStr)
	if defaultTimeout == 0 {
		defaultTimeout = 120 * time.Second
	}
	flag.DurationVar(&cfg.RequestTimeout, "request-timeout", defaultTimeout, "Dify round-trip timeout")

	flag.BoolVar(&cfg.A2AEnabled, "a2a", getEnvBool("A2A_ENABLED", false), "Enable A2A server alongside the proxy")
	flag.IntVar(&cfg.A2APort, "a2a-port", getEnvInt("A2A_PORT", 8000), "A2A server listen port")
	flag.StringVar(&cfg.AgentName, "agent-name", getEnv("AGENT_NAME", "dify-agent"), "A2A AgentCard name")
	flag.StringVar(&cfg.AgentDesc, "agent-desc", getEnv("AGENT_DESC", "Dify-backed agent exposed via A2A protocol"), "A2A AgentCard description")

	flag.Parse()
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	switch v {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
