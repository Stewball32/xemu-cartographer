package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readNetwork reads the per-tick network-game state (client + server +
// machine list + network-player list). Returns nil when neither client nor
// server is reachable.
//
// Source: OffNGC*, OffNGS*, OffNGD*, OffNetMachine*, OffNetPlayer* constants.
func (r *Reader) readNetwork() *scraper.TickNetwork {
	client := r.readNetworkClient()
	server := r.readNetworkServer()
	gameData, machines, players := r.readNetworkGameData()
	if client == nil && server == nil && gameData == nil {
		return nil
	}
	return &scraper.TickNetwork{
		Client:         client,
		Server:         server,
		GameData:       gameData,
		Machines:       machines,
		NetworkPlayers: players,
	}
}

// readNetworkClient reads the network_game_client struct
// (at RefAddrNetworkGameClient). Source: OffNGC* constants.
func (r *Reader) readNetworkClient() *scraper.TickNetworkClient {
	inst := r.inst
	mem := inst.Mem

	baseHVA, err := inst.LowHVA(RefAddrNetworkGameClient)
	if err != nil {
		return nil
	}

	c := &scraper.TickNetworkClient{}
	c.MachineIndex, _ = mem.ReadU16At(baseHVA + int64(OffNGCMachineIndex))
	c.PingTargetIP, _ = mem.ReadS32At(baseHVA + int64(OffNGCPingTargetIP))
	c.PacketsSent, _ = mem.ReadS16At(baseHVA + int64(OffNGCPacketsSent))
	c.PacketsReceived, _ = mem.ReadS16At(baseHVA + int64(OffNGCPacketsReceived))
	c.AveragePing, _ = mem.ReadS16At(baseHVA + int64(OffNGCAveragePing))
	c.PingActive, _ = mem.ReadU8At(baseHVA + int64(OffNGCPingActive))
	c.SecondsToGameStart, _ = mem.ReadS16At(baseHVA + int64(OffNGCSecondsToGameStart))
	return c
}

// readNetworkServer reads the network_game_server struct
// (at RefAddrNetworkGameServer). Source: OffNGS* constants.
func (r *Reader) readNetworkServer() *scraper.TickNetworkServer {
	inst := r.inst
	mem := inst.Mem

	baseHVA, err := inst.LowHVA(RefAddrNetworkGameServer)
	if err != nil {
		return nil
	}

	s := &scraper.TickNetworkServer{}
	s.CountdownActive, _ = mem.ReadU8At(baseHVA + int64(OffNGSCountdownActive))
	s.CountdownPaused, _ = mem.ReadU8At(baseHVA + int64(OffNGSCountdownPaused))
	s.CountdownAdjustedTime, _ = mem.ReadU8At(baseHVA + int64(OffNGSCountdownAdjustedTime))
	return s
}

// readNetworkGameData reads the inline network_game_data sub-struct (at
// network_game_client + 2140) plus the machine + network-player lists.
//
// Source: OffNGD*, OffNetMachine*, OffNetPlayer* constants.
func (r *Reader) readNetworkGameData() (*scraper.TickNetworkGameData, []scraper.TickNetMachine, []scraper.TickNetPlayer) {
	inst := r.inst
	mem := inst.Mem

	clientHVA, err := inst.LowHVA(RefAddrNetworkGameClient)
	if err != nil {
		return nil, nil, nil
	}
	gdHVA := clientHVA + int64(OffNGCNetworkGameData)

	maxPlayers, _ := mem.ReadU8At(gdHVA + int64(OffNGDMaximumPlayerCount))
	machineCount, _ := mem.ReadS16At(gdHVA + int64(OffNGDMachineCount))
	playerCount, _ := mem.ReadS16At(gdHVA + int64(OffNGDPlayerCount))

	gd := &scraper.TickNetworkGameData{
		MaximumPlayerCount: maxPlayers,
		MachineCount:       machineCount,
		PlayerCount:        playerCount,
	}

	var machines []scraper.TickNetMachine
	if machineCount > 0 {
		machinesBase := gdHVA + int64(OffNGDNetworkMachines)
		machines = make([]scraper.TickNetMachine, 0, machineCount)
		for i := int16(0); i < machineCount; i++ {
			entry := machinesBase + int64(i)*int64(NetworkMachineStride)
			nameBytes, _ := mem.ReadBytesAt(entry+int64(OffNetMachineName), 64)
			idx, _ := mem.ReadU8At(entry + int64(OffNetMachineMachineIndex))
			machines = append(machines, scraper.TickNetMachine{
				Index: idx,
				Name:  decodeUTF16LE(nameBytes),
			})
		}
	}

	var players []scraper.TickNetPlayer
	if playerCount > 0 {
		playersBase := gdHVA + int64(OffNGDNetworkPlayers)
		players = make([]scraper.TickNetPlayer, 0, playerCount)
		for i := int16(0); i < playerCount; i++ {
			entry := playersBase + int64(i)*int64(NetworkPlayerStride)
			nameBytes, _ := mem.ReadBytesAt(entry+int64(OffNetPlayerName), 24)
			color, _ := mem.ReadS16At(entry + int64(OffNetPlayerColor))
			unused, _ := mem.ReadS16At(entry + int64(OffNetPlayerUnused))
			machineIdx, _ := mem.ReadU8At(entry + int64(OffNetPlayerMachineIndex))
			ctrlIdx, _ := mem.ReadU8At(entry + int64(OffNetPlayerControllerIndex))
			team, _ := mem.ReadU8At(entry + int64(OffNetPlayerTeam))
			listIdx, _ := mem.ReadU8At(entry + int64(OffNetPlayerListIndex))
			players = append(players, scraper.TickNetPlayer{
				Name:            decodeUTF16LE(nameBytes),
				Color:           color,
				Unused:          unused,
				MachineIndex:    machineIdx,
				ControllerIndex: ctrlIdx,
				Team:            team,
				ListIndex:       listIdx,
			})
		}
	}

	return gd, machines, players
}
