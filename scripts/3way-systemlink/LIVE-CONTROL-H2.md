# Live control cheatsheet — Halo 2 single instance (tag `h2-1`)

Rig dir: `/home/stew/.local/share/xemu/xemu/3way` (= `$T`). Test instance only.
Launched by `launch-h2.sh` (splitscreen-ready: 2 pads, port1+port2).

## Boot / relaunch
    bash $T/launch-h2.sh                      # boots H2 from DVD ISO, visible, hardened
Boot flow (drive via pad FIFO, screenshot each step):
  title "PRESS START" → start → CHOOSE PROFILE (Halo0001) → a → "Don't Sign In" → a
  → main menu (SPLIT SCREEN) → a → P1 "PRESS A TO CONTINUE" → a → PREGAME LOBBY
  → START GAME → a  (solo splitscreen custom game starts; ~15s map load)

## Live memory read (H2) — QMP memsave
    python3 $T/qmp_h2.py /run/user/1000/xemu-qmp-3way-h2-1.sock        # full live state JSON
    python3 $T/qmp_mem.py raw /run/user/1000/xemu-qmp-3way-h2-1.sock 0x4F01F4 4  # any GVA dump
Resolves: title id, players array (@0x4F01F4), objects array (@0x4E78D0), roster
(name/team), biped health/shields/pos/weapons, tag-header scenario (map).

## Screenshots + pads (same tooling as CE rig)
    $T/cap.sh h2-1 /tmp/h2.png
    printf 'start\n' > /run/user/1000/xemu-harness/3way-h2-1/p1.fifo    # player 1
    printf 'a\n'     > /run/user/1000/xemu-harness/3way-h2-1/p2.fifo    # player 2 (splitscreen join)
Vocab: a b x y lb rb back start guide ls rs | up down left right | rt/lt <n> |
move/bwd/sl/sr <s> | looku/lookd/lookl/lookr <s> | aimstep <dx> <dy> | fire <s> | nade <s> | neutral.

## Run the Go scraper against this instance (live integration test)
    cd /home/stew/repos/xemu-cartographer
    H2_QMP_SOCK=/run/user/1000/xemu-qmp-3way-h2-1.sock go test ./internal/scraper/halo2 -run TestLiveH2 -v

## Processes (hardened; survive per-Bash-call teardown)
- xemu: pidfile `$T/logs/xemu-h2-1.pid`, log `$T/logs/xemu-h2-1.out`
- pads: `padpool.py --count 2 --rundir /run/user/1000/xemu-harness/3way-h2-1`
- QMP sock: `/run/user/1000/xemu-qmp-3way-h2-1.sock` (raw; no holder started)
- ptrace shim: LD_PRELOAD `$D/cart-ptrace-shim.so` (enables /proc/<pid>/mem under yama scope=1)

## Stop (tears the H2 instance down)
    kill "$(cat $T/logs/xemu-h2-1.pid)"
    pkill -KILL -f '[p]adpool.py --count .* --rundir /run/user/1000/xemu-harness/3way-h2-1'
