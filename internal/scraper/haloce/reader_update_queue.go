package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readDataQueue reads the data-queue header (at *RefAddrUpdateQueueCounterLo).
// Returns nil when the pointer is unset.
//
// Source: OffDataQueue* constants. The data-queue counter pair Lo/Hi serves
// as a synchronization barrier for input replication; the value at Lo (when
// dereferenced) points to the queue's tick/seed metadata.
func (r *Reader) readDataQueue() *scraper.TickDataQueue {
	inst := r.inst
	mem := inst.Mem

	base, err := inst.DerefLowPtr(RefAddrUpdateQueueCounterLo)
	if err != nil || base < HighGVAThreshold {
		return nil
	}

	tickV, _ := mem.ReadS32(base + OffDataQueueTick)
	rng, _ := mem.ReadU32(base + OffDataQueueGlobalRandom)
	tick2, _ := mem.ReadS32(base + OffDataQueueTick2)
	unk1, _ := mem.ReadU16(base + OffDataQueueUnk1)
	pcount, _ := mem.ReadS16(base + OffDataQueuePlayerCount)

	return &scraper.TickDataQueue{
		Tick:         tickV,
		GlobalRandom: rng,
		Tick2:        tick2,
		Unk1:         unk1,
		PlayerCount:  pcount,
	}
}

// readPlayerUpdateQueue reads one per-player slot from the update_queue array
// at *RefAddrUpdateClientPlayerPtr's first-element pointer + player*0x28.
// Returns nil when the pointer chain isn't yet populated.
//
// Source: OffUQ* constants + OffUQHdr* + UQBtn*/UQAct* bitmasks.
func (r *Reader) readPlayerUpdateQueue(playerIndex int) *scraper.TickUpdateQueue {
	inst := r.inst
	mem := inst.Mem

	hdrBase, err := inst.DerefLowPtr(RefAddrUpdateClientPlayerPtr)
	if err != nil || hdrBase < HighGVAThreshold {
		return nil
	}
	firstElem, _ := mem.ReadU32(hdrBase + OffUQHdrFirstElement)
	if firstElem < HighGVAThreshold {
		return nil
	}

	addr := firstElem + uint32(playerIndex)*UpdateQueuePlayerStride

	uq := &scraper.TickUpdateQueue{}
	uq.UnitRef, _ = mem.ReadU16(addr + OffUQUnitRef)
	uq.ButtonField, _ = mem.ReadU8(addr + OffUQButtonField)
	uq.ActionField, _ = mem.ReadU8(addr + OffUQActionField)
	uq.DesiredYaw, _ = mem.ReadF32(addr + OffUQDesiredYaw)
	uq.DesiredPitch, _ = mem.ReadF32(addr + OffUQDesiredPitch)
	uq.Forward, _ = mem.ReadF32(addr + OffUQForward)
	uq.Left, _ = mem.ReadF32(addr + OffUQLeft)
	uq.RightTriggerHeld, _ = mem.ReadF32(addr + OffUQRightTriggerHeld)
	uq.DesiredWeapon, _ = mem.ReadU16(addr + OffUQDesiredWeapon)
	uq.DesiredGrenades, _ = mem.ReadU16(addr + OffUQDesiredGrenades)
	uq.ZoomLevel, _ = mem.ReadS16(addr + OffUQZoomLevel)

	uq.Buttons = scraper.UpdateQueueButtons{
		Crouch:     uq.ButtonField&UQBtnCrouch != 0,
		Jump:       uq.ButtonField&UQBtnJump != 0,
		Fire:       uq.ButtonField&UQBtnFire != 0,
		Flashlight: uq.ButtonField&UQBtnFlashlight != 0,
		Reload:     uq.ButtonField&UQBtnReload != 0,
		Melee:      uq.ButtonField&UQBtnMelee != 0,
	}
	uq.Actions = scraper.UpdateQueueActions{
		ThrowGrenade: uq.ActionField&UQActThrowGrenade != 0,
		UseAction:    uq.ActionField&UQActUseAction != 0,
	}
	return uq
}
