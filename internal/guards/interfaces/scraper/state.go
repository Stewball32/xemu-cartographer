package scraper

// InstanceState is the per-instance scraper view surfaced to the container
// detail page (game title + Xbox console name + running flag). Empty/zero
// values are normal — they mean the scraper is not yet attached or the
// game-specific reader hasn't resolved that field yet.
type InstanceState struct {
	Name      string `json:"name"`
	TitleID   uint32 `json:"title_id"`
	GameTitle string `json:"game_title"`
	XboxName  string `json:"xbox_name"`
	Running   bool   `json:"running"`
}

// State lets callers fetch a single runner's surfaced state by name.
type State interface {
	InstanceState(name string) (InstanceState, bool)
}
