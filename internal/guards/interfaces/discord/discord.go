package discord

// Service is the aggregate Discord interface.
// Implemented by disgo.Bot via structural typing.
type Service interface {
	Membership
	Roles
	Notify
	Voice
}
