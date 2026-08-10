package config

import (
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	ListenAddr  string
	LogLevel    string
	Env         string
	SiteURL     string
	// CanonicalRedirect enables 301s from non-canonical hosts (e.g. *.fly.dev)
	// to SiteURL. Opt-in via CANONICAL_REDIRECT=1 — turn it on only after DNS
	// for SiteURL is live, or the old host redirects into a dead domain.
	CanonicalRedirect bool
	// FormsURL is the origin of the Cloudflare Worker that receives the report
	// and contact forms (cloudflare/forms). The forms post to it directly, so
	// spam and abuse never reach this server.
	FormsURL string
	// TurnstileSiteKey is the public half of the Turnstile keypair, embedded in
	// the form pages. Empty disables the widget, which is what local
	// development wants: the Worker is the only thing that verifies the token,
	// and it is not in the loop when you are running against localhost.
	TurnstileSiteKey string
}

func Load() Config {
	e := env("ENV", "development")
	dbDefault := ""
	if e == "development" {
		// Local development default, not a credential. Any real deployment sets
		// DATABASE_URL; this branch is unreachable when ENV is not "development".
		dbDefault = "postgres://postgres:postgres@localhost:5432/tenants?sslmode=disable" // #nosec G101
	}
	return Config{
		DatabaseURL: env("DATABASE_URL", dbDefault),
		ListenAddr:  env("LISTEN_ADDR", ":8080"),
		LogLevel:    env("LOG_LEVEL", "info"),
		Env:         e,
		SiteURL:     env("SITE_URL", "https://renterlaw.org"),
		CanonicalRedirect: func() bool {
			v := env("CANONICAL_REDIRECT", "")
			return v == "1" || v == "true"
		}(),
		FormsURL:         env("FORMS_URL", "https://forms.renterlaw.org"),
		TurnstileSiteKey: env("TURNSTILE_SITE_KEY", ""),
	}
}

func (c Config) IsDevelopment() bool {
	return c.Env == "development"
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
