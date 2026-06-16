package routes

import scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"

// scraperSvc is the scraper manager, injected from cmd/server/main.go via
// SetScraper before RegisterAll runs. Typed against the Service interface so
// the routes package keeps no compile-time dependency on the manager. nil when
// no scraper subsystem is running — match resolution then reports "no match".
var scraperSvc scraperiface.Service

// SetScraper wires the scraper manager for routes that resolve player identity
// against live container rosters (M09 /api/me/match). Must be called before
// RegisterAll; safe to leave unset.
func SetScraper(s scraperiface.Service) {
	scraperSvc = s
}
