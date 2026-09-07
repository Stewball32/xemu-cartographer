package scraper

import "github.com/xemu-cartographer/xc-scraper/runner"

// MapOption is one selectable map (or gametype) enumerated LIVE from a specific
// instance. Alias of runner.MapOption (moved in step 7 part 3c).
type MapOption = runner.MapOption

// MapList is the per-instance available maps + gametypes the player API serves
// (with the IndexOf helper). Alias of runner.MapList.
type MapList = runner.MapList
