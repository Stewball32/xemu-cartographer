package lansaves

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// transport carries the non-content download parameters: the target free space
// to check against, which file(s)/format to serve, and an optional FATX cluster
// override for the footprint math.
type transport struct {
	FreeBytes   *uint64 // target (Xbox) free bytes; nil = client did not report
	HasFree     bool
	Format      string // "tar" | "zip" | "payload" | "savemeta" | "file"
	File        string // for Format=="file": the exact filename
	ClusterSize int    // FATX cluster override; 0 = default
}

// jsonBody is the POST shape: a BuildRequest plus the transport fields. The
// embedded BuildRequest's json tags are promoted, so a single flat JSON object
// like {"title":"ce","kind":"gametype","name":"TS 25","free_bytes":8388608}
// binds correctly.
type jsonBody struct {
	halosave.BuildRequest
	FreeBytes   *uint64 `json:"free_bytes"`
	Format      string  `json:"format"`
	File        string  `json:"file"`
	ClusterSize int     `json:"cluster_size"`
}

// specFromQuery maps URL query params onto a BuildRequest + transport. Used by
// the GET form (the nxdk LAN client). Unknown params are ignored.
//
//	title, kind, name, internal_name, dir_name, engine    (strings)
//	teams, radar, friend_indicators, infinite_grenades,
//	  shields_off, invisible_players, generic_equipment,
//	  odd_man_out, death_bonus_off, kill_penalty_off,
//	  kill_in_order, assault, flag_must_reset, flag_at_home,
//	  moving_hill, random_start, race_any_order, recompute   (bools: 1/true/yes/on)
//	objectives_indicator, lives, weapon_set, nhe_toggles,
//	  score_limit, oddball_speed, oddball_trait_with,
//	  oddball_trait_without, oddball_ball_type, ball_spawn_count,
//	  race_scoring, options, engine_union                    (uint32)
//	respawn_seconds, respawn_growth_seconds, suicide_seconds,
//	  max_health, ctf_single_flag_minutes                    (float)
//	app_<key>=<int>                                          (H2 appearance bytes)
//	free_bytes (uint64), format, file, cluster_size (int)
func specFromQuery(q url.Values) (halosave.BuildRequest, transport) {
	req := halosave.BuildRequest{
		Title:        q.Get("title"),
		Kind:         q.Get("kind"),
		Name:         q.Get("name"),
		InternalName: q.Get("internal_name"),
		DirName:      q.Get("dir_name"),
		Engine:       q.Get("engine"),
	}
	if b := boolPtr(q, "recompute"); b != nil {
		req.Recompute = *b
	}
	req.Teams = boolPtr(q, "teams")
	req.Radar = boolPtr(q, "radar")
	req.FriendIndicators = boolPtr(q, "friend_indicators")
	req.InfiniteGrenades = boolPtr(q, "infinite_grenades")
	req.ShieldsOff = boolPtr(q, "shields_off")
	req.InvisiblePlayers = boolPtr(q, "invisible_players")
	req.GenericEquipment = boolPtr(q, "generic_equipment")
	req.Options = u32Ptr(q, "options")

	req.ObjectivesIndicator = u32Ptr(q, "objectives_indicator")
	req.OddManOut = boolPtr(q, "odd_man_out")
	req.RespawnSeconds = floatPtr(q, "respawn_seconds")
	req.RespawnGrowthSeconds = floatPtr(q, "respawn_growth_seconds")
	req.SuicideSeconds = floatPtr(q, "suicide_seconds")
	req.Lives = u32Ptr(q, "lives")
	req.MaxHealth = float32Ptr(q, "max_health")
	req.ScoreLimit = u32Ptr(q, "score_limit")
	req.WeaponSet = u32Ptr(q, "weapon_set")
	req.NHEToggles = u32Ptr(q, "nhe_toggles")
	req.EngineUnion = u32Ptr(q, "engine_union")

	req.DeathBonusOff = boolPtr(q, "death_bonus_off")
	req.KillPenaltyOff = boolPtr(q, "kill_penalty_off")
	req.KillInOrder = boolPtr(q, "kill_in_order")
	req.Assault = boolPtr(q, "assault")
	req.FlagMustReset = boolPtr(q, "flag_must_reset")
	req.FlagAtHome = boolPtr(q, "flag_at_home")
	req.MovingHill = boolPtr(q, "moving_hill")
	req.RandomStart = boolPtr(q, "random_start")
	req.RaceAnyOrder = boolPtr(q, "race_any_order")

	req.CTFSingleFlagMinutes = floatPtr(q, "ctf_single_flag_minutes")
	req.OddballSpeed = u32Ptr(q, "oddball_speed")
	req.OddballTraitWith = u32Ptr(q, "oddball_trait_with")
	req.OddballTraitWithout = u32Ptr(q, "oddball_trait_without")
	req.OddballBallType = u32Ptr(q, "oddball_ball_type")
	req.BallSpawnCount = u32Ptr(q, "ball_spawn_count")
	req.RaceScoring = u32Ptr(q, "race_scoring")

	// CE profile
	req.Color = u32Ptr(q, "color")
	req.Button = u32Ptr(q, "button")
	req.Thumbstick = u32Ptr(q, "thumbstick")
	req.HSens = floatPtr(q, "h_sens")
	req.VMult = floatPtr(q, "v_mult")
	req.Invert = boolPtr(q, "invert")
	req.Vibration = boolPtr(q, "vibration")
	req.RSDeadzone = u32Ptr(q, "rs_deadzone")
	req.LSDeadzone = u32Ptr(q, "ls_deadzone")
	req.OuterDeadzone = u32Ptr(q, "outer_deadzone")
	req.DeadzoneType = u32Ptr(q, "deadzone_type")
	req.Response = u32Ptr(q, "response")

	for key := range q {
		if strings.HasPrefix(key, "app_") {
			if v, err := strconv.Atoi(q.Get(key)); err == nil {
				if req.Appearance == nil {
					req.Appearance = map[string]int{}
				}
				req.Appearance[strings.TrimPrefix(key, "app_")] = v
			}
		}
	}

	tr := transport{
		Format:      q.Get("format"),
		File:        q.Get("file"),
		ClusterSize: atoiDefault(q.Get("cluster_size"), 0),
	}
	if fb := u64Ptr(q, "free_bytes"); fb != nil {
		tr.FreeBytes = fb
		tr.HasFree = true
	}
	return req, tr
}

// specFromBody maps a decoded jsonBody onto a BuildRequest + transport.
func specFromBody(b jsonBody) (halosave.BuildRequest, transport) {
	tr := transport{Format: b.Format, File: b.File, ClusterSize: b.ClusterSize}
	if b.FreeBytes != nil {
		tr.FreeBytes = b.FreeBytes
		tr.HasFree = true
	}
	return b.BuildRequest, tr
}

func boolPtr(q url.Values, key string) *bool {
	if !q.Has(key) {
		return nil
	}
	switch strings.ToLower(q.Get(key)) {
	case "1", "true", "yes", "on":
		t := true
		return &t
	default:
		f := false
		return &f
	}
}

func u32Ptr(q url.Values, key string) *uint32 {
	if !q.Has(key) {
		return nil
	}
	// base 0 auto-detects a 0x prefix (handy for the options bitfield / union).
	v, err := strconv.ParseUint(q.Get(key), 0, 32)
	if err != nil {
		return nil
	}
	u := uint32(v)
	return &u
}

func u64Ptr(q url.Values, key string) *uint64 {
	if !q.Has(key) {
		return nil
	}
	v, err := strconv.ParseUint(q.Get(key), 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func floatPtr(q url.Values, key string) *float64 {
	if !q.Has(key) {
		return nil
	}
	v, err := strconv.ParseFloat(q.Get(key), 64)
	if err != nil {
		return nil
	}
	return &v
}

func float32Ptr(q url.Values, key string) *float32 {
	if !q.Has(key) {
		return nil
	}
	v, err := strconv.ParseFloat(q.Get(key), 32)
	if err != nil {
		return nil
	}
	f := float32(v)
	return &f
}

func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
