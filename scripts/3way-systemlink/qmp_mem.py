#!/usr/bin/env python3
"""qmp_mem.py — read Halo CE live state from an xemu QMP unix socket via memsave.

memsave takes a guest VIRTUAL address; CE engine offsets (0x2Exxxx/0x2Fxxxx) and
high EEPROM GVAs (0x80056F80) are fed directly. We dump bytes to a temp file via
HMP `memsave` (the only mem command this xemu build exposes) and parse them.

Subcommands:
  raw   SOCK ADDR LEN              -> hexdump bytes
  ce    SOCK                       -> full CE system-link state (JSON)
  state SOCK                       -> one-line scene gate (menu vs in-game), for nav

CE state read (all addresses guest-virtual):
  game_connection  u16 @0x2E3684   (0 menu/SP, 1 syslink, 2 hosting, 3 film)
  main_menu_active u8  @0x2E4068   (1 at menu, 0 in game)
  network_game_client struct @0x2FB180 (direct global; network_game_data inline +2140)
    machine_count s16 @ +2140+274 = 0x2FB8CE
    player_count  s16 @ +2140+548 = 0x2FB9D8
    machines: base +2140+276 = 0x2FB8D0, stride 68, name UTF-16LE[64]@+0, idx u8@+64
    players : base +2140+550 = 0x2FB9DA, stride 32, name UTF-16LE[24]@+0,
              color s16@24, machine_index u8@28, ctrl u8@29, team u8@30, list u8@31
  network_game_server @0x2FBE40: variant name UTF-16LE[24]@0xAC, gametype u32@0xC4, teamplay u32@0xC8
  map: deref 0x2DF1C4 -> hdr; idx=(u32@hdr+4)&0xFFFF; ta=u32@hdr+0; nameptr=u32@(ta+idx*0x20+0x10); cstring
  mac: 6 bytes @0x80056F80
"""
import socket, json, struct, sys, os, time, tempfile

class QMP:
    def __init__(self, sock_path, timeout=5.0):
        self.s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.s.settimeout(timeout)
        self.s.connect(sock_path)
        self.buf = b""
        self._readline()                      # greeting
        self._cmd({"execute": "qmp_capabilities"})
    def _readline(self):
        while b"\n" not in self.buf:
            d = self.s.recv(65536)
            if not d:
                break
            self.buf += d
        line, _, self.buf = self.buf.partition(b"\n")
        return line
    def _cmd(self, obj):
        self.s.sendall((json.dumps(obj) + "\r\n").encode())
        while True:
            line = self._readline()
            if not line:
                return None
            try:
                m = json.loads(line)
            except Exception:
                continue
            if "event" in m:                  # skip async events
                continue
            return m
    def hmp(self, cmdline):
        r = self._cmd({"execute": "human-monitor-command",
                       "arguments": {"command-line": cmdline}})
        return (r or {}).get("return", "")
    def memsave(self, addr, length):
        f = tempfile.NamedTemporaryFile(prefix="qmpmem_", suffix=".bin", delete=False)
        path = f.name; f.close()
        self.hmp('memsave 0x%X %d "%s"' % (addr, length, path))
        for _ in range(50):
            try:
                if os.path.getsize(path) >= length:
                    break
            except OSError:
                pass
            time.sleep(0.01)
        with open(path, "rb") as fh:
            data = fh.read()
        try: os.unlink(path)
        except OSError: pass
        return data
    def u8(self, a):  d=self.memsave(a,1); return d[0] if len(d)>=1 else None
    def u16(self, a): d=self.memsave(a,2); return struct.unpack("<H",d)[0] if len(d)>=2 else None
    def s16(self, a): d=self.memsave(a,2); return struct.unpack("<h",d)[0] if len(d)>=2 else None
    def u32(self, a): d=self.memsave(a,4); return struct.unpack("<I",d)[0] if len(d)>=4 else None
    def close(self):
        try: self.s.close()
        except Exception: pass

def utf16(data):
    try:
        return data.decode("utf-16-le", "ignore").split("\x00")[0]
    except Exception:
        return ""

def cstr(q, ptr, maxlen=64):
    if not ptr or ptr < 0x10000:
        return ""
    d = q.memsave(ptr, maxlen)
    z = d.find(b"\x00")
    return d[:z if z >= 0 else maxlen].decode("ascii", "ignore")

def read_ce(sock):
    q = QMP(sock)
    out = {"sock": sock, "ts": round(time.time(), 2)}
    gc = q.u16(0x2E3684); mm = q.u8(0x2E4068)
    out["game_connection"] = gc
    out["game_connection_label"] = {0:"menu/SP",1:"syslink-join",2:"hosting",3:"film"}.get(gc, "?")
    out["main_menu_active"] = mm
    # network_game_client is a direct global struct at 0x2FB180; ngd inline at +2140
    NGD = 0x2FB180 + 2140
    out["machine_count"] = q.s16(NGD + 274)
    out["player_count"]  = q.s16(NGD + 548)
    out["own_machine_index"] = q.u16(0x2FB180 + 0)
    # machine roster (consoles)
    machines = []
    mc = out["machine_count"] or 0
    if 0 < mc <= 8:
        for k in range(mc):
            base = NGD + 276 + k*68
            machines.append({"i": k, "name": utf16(q.memsave(base, 64)),
                             "machine_index": q.u8(base + 64)})
    out["machines"] = machines
    # player roster
    players = []
    pc = out["player_count"] or 0
    if 0 < pc <= 16:
        for k in range(pc):
            base = NGD + 550 + k*32
            players.append({"i": k, "name": utf16(q.memsave(base, 24)),
                            "color": q.s16(base+24), "machine_index": q.u8(base+28),
                            "controller": q.u8(base+29), "team": q.u8(base+30),
                            "list_index": q.u8(base+31)})
    out["players"] = players
    # host variant (network_game_server)
    NGS = 0x2FBE40
    out["variant_name"] = utf16(q.memsave(NGS + 0xAC, 24))
    out["variant_gametype"] = q.u32(NGS + 0xC4)
    out["variant_teamplay"] = q.u32(NGS + 0xC8)
    # map via tag header
    try:
        hdr = q.u32(0x2DF1C4)
        if hdr and hdr >= 0x10000:
            scen = q.u32(hdr + 0x04)
            idx = scen & 0xFFFF
            ta = q.u32(hdr + 0x00)
            namep = q.u32(ta + idx*0x20 + 0x10) if ta else 0
            out["map"] = cstr(q, namep, 96)
            out["tag_count"] = q.u32(hdr + 0x0C)
        else:
            out["map"] = ""
    except Exception as ex:
        out["map"] = "ERR:%r" % ex
    # MAC (high GVA)
    mac = q.memsave(0x80056F80, 6)
    out["mac"] = ":".join("%02x" % b for b in mac) if len(mac) == 6 else None
    q.close()
    return out

def main():
    if len(sys.argv) < 3:
        sys.exit("usage: qmp_mem.py {raw|ce|state} SOCK [ADDR LEN]")
    cmd, sock = sys.argv[1], sys.argv[2]
    if cmd == "raw":
        addr = int(sys.argv[3], 0); length = int(sys.argv[4], 0)
        q = QMP(sock); d = q.memsave(addr, length); q.close()
        print("addr=0x%X len=%d" % (addr, length))
        for off in range(0, len(d), 16):
            chunk = d[off:off+16]
            hexs = " ".join("%02x" % b for b in chunk)
            asc = "".join(chr(b) if 32 <= b < 127 else "." for b in chunk)
            print("  %04x  %-47s  %s" % (off, hexs, asc))
    elif cmd == "ce":
        print(json.dumps(read_ce(sock), indent=2))
    elif cmd == "state":
        q = QMP(sock); mm = q.u8(0x2E4068); gc = q.u16(0x2E3684); q.close()
        print("main_menu_active=%s game_connection=%s" % (mm, gc))
    else:
        sys.exit("unknown cmd %s" % cmd)

if __name__ == "__main__":
    main()
