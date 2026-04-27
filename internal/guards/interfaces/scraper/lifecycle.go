package scraper

// Lifecycle abstracts scraper start/stop. Used by the discovery watcher
// (auto-start on QMP socket appearance) and the /api/admin/scraper/* routes
// (manual start/stop).
type Lifecycle interface {
	Start(name, sock string) error
	Stop(name string) error
}
