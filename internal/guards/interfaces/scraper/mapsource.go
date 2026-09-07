package scraper

import "github.com/Stewball32/xemu-cartographer/internal/scraper/manager"

// MapOption is one selectable map (or gametype) enumerated LIVE from a specific
// instance. Alias of manager.MapOption (moved in step 7 part 3c).
type MapOption = manager.MapOption

// MapList is the per-instance available maps + gametypes the player API serves
// (with the IndexOf helper). Alias of manager.MapList.
type MapList = manager.MapList
