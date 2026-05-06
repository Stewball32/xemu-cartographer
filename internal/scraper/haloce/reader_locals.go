package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// MaxLocalPlayers is the engine's hard cap on splitscreen locals.
const MaxLocalPlayers = 4

// readLocals iterates 0..localCount-1 and assembles the per-local-player
// subsystem state (FPW, observer cam, IAS, gamepad, UI, player control,
// look rates). UI globals and look rates are pulled from r.matchCache —
// they're match-static and pre-read by ensureMatchStatic. The remaining
// fields are live per-tick reads.
//
// Update-queue slots are filled by readPlayerUpdateQueue elsewhere.
func (r *Reader) readLocals(localCount uint16) []scraper.TickLocal {
	if localCount == 0 {
		return nil
	}
	if localCount > MaxLocalPlayers {
		localCount = MaxLocalPlayers
	}

	out := make([]scraper.TickLocal, 0, localCount)
	for i := uint16(0); i < localCount; i++ {
		idx := int(i)
		l := scraper.TickLocal{
			LocalIndex:    idx,
			FPWeapon:      r.readFPWeapon(idx),
			ObserverCam:   r.readObserverCam(idx),
			IAS:           r.readInputAbstract(idx),
			Gamepad:       r.readGamepad(idx),
			UI:            r.cachedUI(idx),
			PlayerControl: r.readPlayerControl(idx),
			LookYawRate:   r.cachedLookYaw(idx),
			LookPitchRate: r.cachedLookPitch(idx),
		}
		out = append(out, l)
	}
	return out
}

// cachedUI returns matchCache.UI[idx] when filled; falls back to a live read
// in case the cache hasn't populated yet (e.g. ReadTick called before any
// ensureMatchStatic completed). Same fallback applies to cachedLookYaw /
// cachedLookPitch below.
func (r *Reader) cachedUI(idx int) *scraper.TickUIGlobals {
	if r.matchCache != nil && idx < len(r.matchCache.UI) {
		return r.matchCache.UI[idx]
	}
	return r.readUIGlobals(idx)
}

func (r *Reader) cachedLookYaw(idx int) float32 {
	if r.matchCache != nil && idx < len(r.matchCache.LookYawRate) {
		return r.matchCache.LookYawRate[idx]
	}
	return r.readLookRate(RefAddrLookYawRate, idx)
}

func (r *Reader) cachedLookPitch(idx int) float32 {
	if r.matchCache != nil && idx < len(r.matchCache.LookPitchRate) {
		return r.matchCache.LookPitchRate[idx]
	}
	return r.readLookRate(RefAddrLookPitchRate, idx)
}

// readFPWeapon reads the first-person weapon record for one local player.
// Located at *RefAddrFPWeaponPtr + 7840*local. Source: OffFPW* constants.
func (r *Reader) readFPWeapon(local int) *scraper.TickFPWeapon {
	inst := r.inst
	mem := inst.Mem

	base, err := inst.DerefLowPtr(RefAddrFPWeaponPtr)
	if err != nil || base < HighGVAThreshold {
		return nil
	}
	addr := base + uint32(local)*FPWeaponStride

	rendered, _ := mem.ReadU32(addr + OffFPWWeaponRendered)
	playerObj, _ := mem.ReadU32(addr + OffFPWPlayerObject)
	weaponObj, _ := mem.ReadU32(addr + OffFPWWeaponObject)
	state, _ := mem.ReadS16(addr + OffFPWState)
	idleThr, _ := mem.ReadS16(addr + OffFPWIdleAnimationThreshold)
	idleCnt, _ := mem.ReadS16(addr + OffFPWIdleAnimationCounter)
	animID, _ := mem.ReadS16(addr + OffFPWAnimationID)
	animTick, _ := mem.ReadS16(addr + OffFPWAnimationTick)

	return &scraper.TickFPWeapon{
		WeaponRendered:         rendered,
		PlayerObject:           playerObj,
		WeaponObject:           weaponObj,
		State:                  state,
		IdleAnimationThreshold: idleThr,
		IdleAnimationCounter:   idleCnt,
		AnimationID:            animID,
		AnimationTick:          animTick,
	}
}

// readObserverCam reads the observer-camera record for one local player.
// Located at RefAddrObserverCameraBase + 668*local. Source: OffObsCam* constants.
func (r *Reader) readObserverCam(local int) *scraper.TickObserverCam {
	inst := r.inst
	mem := inst.Mem

	baseHVA, err := inst.LowHVA(RefAddrObserverCameraBase)
	if err != nil {
		return nil
	}
	addrHVA := baseHVA + int64(local)*int64(ObserverCameraStride)

	x, _ := mem.ReadF32At(addrHVA + int64(OffObsCamX))
	y, _ := mem.ReadF32At(addrHVA + int64(OffObsCamY))
	z, _ := mem.ReadF32At(addrHVA + int64(OffObsCamZ))
	vx, _ := mem.ReadF32At(addrHVA + int64(OffObsCamVelX))
	vy, _ := mem.ReadF32At(addrHVA + int64(OffObsCamVelY))
	vz, _ := mem.ReadF32At(addrHVA + int64(OffObsCamVelZ))
	ax, _ := mem.ReadF32At(addrHVA + int64(OffObsCamAimX))
	ay, _ := mem.ReadF32At(addrHVA + int64(OffObsCamAimY))
	az, _ := mem.ReadF32At(addrHVA + int64(OffObsCamAimZ))
	fov, _ := mem.ReadF32At(addrHVA + int64(OffObsCamFOV))

	return &scraper.TickObserverCam{
		X: x, Y: y, Z: z,
		VelX: vx, VelY: vy, VelZ: vz,
		AimX: ax, AimY: ay, AimZ: az,
		FOV: fov,
	}
}

// readInputAbstract reads the post-button-config input state for one local
// player. Located at RefAddrInputAbstractInputState + 0x1C*local.
// Source: OffIAS* constants.
func (r *Reader) readInputAbstract(local int) *scraper.TickInputAbstract {
	inst := r.inst
	mem := inst.Mem

	baseHVA, err := inst.LowHVA(RefAddrInputAbstractInputState)
	if err != nil {
		return nil
	}
	addrHVA := baseHVA + int64(local)*int64(InputAbstractStateStride)

	ias := &scraper.TickInputAbstract{}
	ias.BtnA, _ = mem.ReadU8At(addrHVA + int64(OffIASBtnA))
	ias.BtnBlack, _ = mem.ReadU8At(addrHVA + int64(OffIASBtnBlack))
	ias.BtnX, _ = mem.ReadU8At(addrHVA + int64(OffIASBtnX))
	ias.BtnY, _ = mem.ReadU8At(addrHVA + int64(OffIASBtnY))
	ias.BtnB, _ = mem.ReadU8At(addrHVA + int64(OffIASBtnB))
	ias.BtnWhite, _ = mem.ReadU8At(addrHVA + int64(OffIASBtnWhite))
	ias.LeftTrigger, _ = mem.ReadU8At(addrHVA + int64(OffIASLeftTrigger))
	ias.RightTrigger, _ = mem.ReadU8At(addrHVA + int64(OffIASRightTrigger))
	ias.BtnStart, _ = mem.ReadU8At(addrHVA + int64(OffIASBtnStart))
	ias.BtnBack, _ = mem.ReadU8At(addrHVA + int64(OffIASBtnBack))
	ias.LeftStickButton, _ = mem.ReadU8At(addrHVA + int64(OffIASLeftStickButton))
	ias.RightStickButton, _ = mem.ReadU8At(addrHVA + int64(OffIASRightStickButton))
	ias.LeftStickVertical, _ = mem.ReadF32At(addrHVA + int64(OffIASLeftStickVertical))
	ias.LeftStickHorizontal, _ = mem.ReadF32At(addrHVA + int64(OffIASLeftStickHorizontal))
	ias.RightStickHorizontal, _ = mem.ReadF32At(addrHVA + int64(OffIASRightStickHorizontal))
	ias.RightStickVertical, _ = mem.ReadF32At(addrHVA + int64(OffIASRightStickVertical))
	return ias
}

// readGamepad reads the raw-controller gamepad record for one player.
// Located at RefAddrGamepadState + 0x28*player. Source: OffGP* constants.
func (r *Reader) readGamepad(player int) *scraper.TickGamepad {
	inst := r.inst
	mem := inst.Mem

	baseHVA, err := inst.LowHVA(RefAddrGamepadState)
	if err != nil {
		return nil
	}
	addrHVA := baseHVA + int64(player)*int64(GamepadStateStride)

	gp := &scraper.TickGamepad{}
	gp.BtnA, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnA))
	gp.BtnB, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnB))
	gp.BtnX, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnX))
	gp.BtnY, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnY))
	gp.BtnBlack, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnBlack))
	gp.BtnWhite, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnWhite))
	gp.LeftTrigger, _ = mem.ReadU8At(addrHVA + int64(OffGPLeftTrigger))
	gp.RightTrigger, _ = mem.ReadU8At(addrHVA + int64(OffGPRightTrigger))
	gp.BtnADuration, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnADuration))
	gp.BtnBDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnBDuration))
	gp.BtnXDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnXDuration))
	gp.BtnYDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPBtnYDuration))
	gp.BlackDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPBlackDuration))
	gp.WhiteDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPWhiteDuration))
	gp.LTDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPLTDuration))
	gp.RTDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPRTDuration))
	gp.DpadUpDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPDpadUpDuration))
	gp.DpadDownDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPDpadDownDuration))
	gp.DpadLeftDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPDpadLeftDuration))
	gp.DpadRightDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPDpadRightDuration))
	gp.LeftStickDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPLeftStickDuration))
	gp.RightStickDuration, _ = mem.ReadU8At(addrHVA + int64(OffGPRightStickDuration))
	gp.LeftStickHorizontal, _ = mem.ReadS16At(addrHVA + int64(OffGPLeftStickHorizontal))
	gp.LeftStickVertical, _ = mem.ReadS16At(addrHVA + int64(OffGPLeftStickVertical))
	gp.RightStickHorizontal, _ = mem.ReadS16At(addrHVA + int64(OffGPRightStickHorizontal))
	gp.RightStickVertical, _ = mem.ReadS16At(addrHVA + int64(OffGPRightStickVertical))
	return gp
}

// readUIGlobals reads the per-local UI/profile config for one local player.
// Located at RefAddrPerLocalUIGlobals + 56*local. Source: OffUI* constants.
func (r *Reader) readUIGlobals(local int) *scraper.TickUIGlobals {
	inst := r.inst
	mem := inst.Mem

	baseHVA, err := inst.LowHVA(RefAddrPerLocalUIGlobals)
	if err != nil {
		return nil
	}
	addrHVA := baseHVA + int64(local)*int64(PerLocalUIStride)

	ui := &scraper.TickUIGlobals{}
	ui.Color, _ = mem.ReadU8At(addrHVA + int64(OffUIColor))
	ui.ButtonConfig, _ = mem.ReadU8At(addrHVA + int64(OffUIButtonConfig))
	ui.JoystickConfig, _ = mem.ReadU8At(addrHVA + int64(OffUIJoystickConfig))
	ui.Sensitivity, _ = mem.ReadU8At(addrHVA + int64(OffUISensitivity))
	ui.JoystickInverted, _ = mem.ReadU8At(addrHVA + int64(OffUIJoystickInverted))
	ui.RumbleEnabled, _ = mem.ReadU8At(addrHVA + int64(OffUIRumbleEnabled))
	ui.FlightInverted, _ = mem.ReadU8At(addrHVA + int64(OffUIFlightInverted))
	ui.AutocenterEnabled, _ = mem.ReadU8At(addrHVA + int64(OffUIAutocenterEnabled))
	ui.ActivePlayerProfileIndex, _ = mem.ReadU32At(addrHVA + int64(OffUIActivePlayerProfileIndex))
	ui.JoinedMultiplayerGame, _ = mem.ReadU8At(addrHVA + int64(OffUIJoinedMultiplayerGame))
	return ui
}

// readPlayerControl reads the player_control struct for one local player.
// Located at *RefAddrPlayerControlPtr + (local << 6). Source: OffPC* constants.
func (r *Reader) readPlayerControl(local int) *scraper.TickPlayerControl {
	inst := r.inst
	mem := inst.Mem

	base, err := inst.DerefLowPtr(RefAddrPlayerControlPtr)
	if err != nil || base < HighGVAThreshold {
		return nil
	}
	addr := base + uint32(local)<<6

	pc := &scraper.TickPlayerControl{}
	pc.DesiredYaw, _ = mem.ReadF32(addr + OffPCDesiredYaw)
	pc.DesiredPitch, _ = mem.ReadF32(addr + OffPCDesiredPitch)
	pc.ZoomLevel, _ = mem.ReadS16(addr + OffPCZoomLevel)
	pc.AimAssistTarget, _ = mem.ReadU32(addr + OffPCAimAssistTarget)
	pc.AimAssistNear, _ = mem.ReadF32(addr + OffPCAimAssistNear)
	pc.AimAssistFar, _ = mem.ReadF32(addr + OffPCAimAssistFar)
	return pc
}

// readLookRate reads one f32 from a 4-byte-stride per-local table.
// Used for both RefAddrLookYawRate and RefAddrLookPitchRate.
func (r *Reader) readLookRate(baseGVA uint32, local int) float32 {
	inst := r.inst
	baseHVA, err := inst.LowHVA(baseGVA)
	if err != nil {
		return 0
	}
	v, _ := inst.Mem.ReadF32At(baseHVA + int64(local)*4)
	return v
}
