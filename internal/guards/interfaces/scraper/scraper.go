package scraper

// Service is the aggregate scraper-manager interface. Implemented by
// internal/leaguescraper.WireAdapter — which embeds
// xc-scraper/runner.Manager and frames its request/reply envelopes
// (runner.Reply) as wire.Messages — via structural typing. The view types
// (Info, InspectState, InstanceState, MapList, ContainerMembership, …) are
// aliases of the manager's own since step 7 part 3c; the manager does not
// import this package.
type Service interface {
	Lifecycle
	Inspect
	JoinReplay
	EventsReply
	ProbeReply
	State
	Membership
}
