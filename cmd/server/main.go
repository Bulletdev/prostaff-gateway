package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"prostaff-riot-gateway/internal/auth"
	"prostaff-riot-gateway/internal/cache"
	"prostaff-riot-gateway/internal/circuit"
	"prostaff-riot-gateway/internal/config"
	"prostaff-riot-gateway/internal/handlers"
	"prostaff-riot-gateway/internal/ratelimit"
	"prostaff-riot-gateway/internal/riot"
)

func main() {
	cfg := config.Load()

	logger := buildLogger(cfg.LogLevel)

	l1 := cache.NewMemory(60 * time.Second)
	l2 := cache.NewRedis(cfg.RedisURL, cfg.CacheEnabled, logger)

	limiter := ratelimit.NewRegionLimiter(cfg.RiotRateLimitPerSecond, cfg.RiotRateLimitBurst)
	breakers := circuit.NewRegionBreakers(cfg.CBThreshold, cfg.CBTimeout, cfg.CBCooldown)

	riotClient := riot.NewClient(cfg.RiotAPITimeout, cfg.RiotAPIKey, limiter, breakers, logger)

	summonerH := handlers.NewSummonerHandler(riotClient, l1, l2, logger)
	leagueH := handlers.NewLeagueHandler(riotClient, l1, l2, logger)
	matchesH := handlers.NewMatchesHandler(riotClient, l1, l2, logger)
	masteryH := handlers.NewMasteryHandler(riotClient, l1, l2, logger)
	healthH := handlers.NewHealthHandler(breakers, l2)

	r := buildRouter(summonerH, leagueH, matchesH, masteryH, healthH, cfg.InternalJWTSecret)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("prostaff-riot-gateway started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down gracefully")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("server stopped")
}

func buildRouter(
	summonerH *handlers.SummonerHandler,
	leagueH *handlers.LeagueHandler,
	matchesH *handlers.MatchesHandler,
	masteryH *handlers.MasteryHandler,
	healthH *handlers.HealthHandler,
	jwtSecret string,
) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/health", healthH.Handle).Methods(http.MethodGet)

	riot := r.PathPrefix("/riot").Subrouter()
	riot.Use(auth.InternalAuth(jwtSecret))

	riot.HandleFunc("/summoner/{region}/by-puuid/{puuid}", summonerH.ByPUUID).Methods(http.MethodGet)
	riot.HandleFunc("/summoner/{region}/by-name/{name}", summonerH.ByName).Methods(http.MethodGet)
	riot.HandleFunc("/account/{region}/{riotId}/{tagline}", summonerH.AccountByRiotID).Methods(http.MethodGet)
	riot.HandleFunc("/account/{region}/by-puuid/{puuid}", summonerH.AccountByPUUID).Methods(http.MethodGet)

	riot.HandleFunc("/league/{region}/by-summoner/{summonerId}", leagueH.BySummoner).Methods(http.MethodGet)
	riot.HandleFunc("/league/{region}/by-puuid/{puuid}", leagueH.ByPUUID).Methods(http.MethodGet)

	riot.HandleFunc("/matches/{region}/{puuid}/ids", matchesH.IDs).Methods(http.MethodGet)
	riot.HandleFunc("/match/{region}/{matchId}", matchesH.Detail).Methods(http.MethodGet)

	riot.HandleFunc("/mastery/{region}/{puuid}/top", masteryH.Top).Methods(http.MethodGet)

	return r
}

func buildLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
