package scraper

// Service is the aggregate scraper-manager interface. Implemented by
// internal/scraper/manager.Manager via structural typing — no compile-time
// dependency between the manager and this package.
type Service interface {
	Lifecycle
	Inspect
	Snapshot
}
