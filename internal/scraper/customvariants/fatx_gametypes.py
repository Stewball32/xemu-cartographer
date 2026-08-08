#!/usr/bin/env python3
"""Enumerate a box's SAVED custom gametype variants off its guest HDD.

Host-side reader for the one thing the guest can't surface in memory: the FULL
set of user-saved CE gametype variants (the SELECT GAMETYPE carousel keeps only
the rendered cards resident and streams the rest from disk). Reads them from the
box's overlay qcow2 the same proven way the testrig does — CoW-snapshot the live
overlay (reflink; never lock the running xemu), FUSE-export it, pyfatx-list
E:\\UDATA\\<title>\\, and read each blam.lst's in-file name (@0x00, UTF-16LE).

Output (stdout): JSON {"names": [...]} — the custom variant display names in
carousel order (sorted directory name; the pack's dir prefixes control menu
order, RUNTIME-VALIDATED to match the live carousel). Each name is signature-
verified against the roamable-save HMAC before it is emitted, so a corrupt /
foreign file can never inject a garbage name.

Usage: fatx_gametypes.py <overlay.qcow2> <titleID hex, e.g. 4d530004>
Exit 0 with {"names":[...]} on success (possibly empty); non-zero + {"error":..}
on any failure so the caller can fail SOFT (fall back to built-ins only).
"""
import contextlib, hashlib, hmac, json, os, shutil, subprocess, sys, tempfile, time

# Roamable-save signature (Original Xbox), per-title, console-independent — the
# same constants halosave.py / ceprofile.py use. A CE gametype variant signs
# blam.lst[0x00:0x68] with digest @ 0x68.
XBOX_GLOBAL_KEY = bytes.fromhex("5C0733AE0401F7E8BA7993FDCD2F1FE0")
TITLE_CE_SIG_KEY = bytes.fromhex("1F71DE93D52AADB19446D7494F731158")
SIG_OFF = 0x68
NAME_OFF, NAME_LEN = 0x00, 0x18  # 24-byte UTF-16LE name buffer


def _digest_ok(buf: bytes) -> bool:
    if len(buf) < SIG_OFF + 20:
        return False
    auth = hmac.new(XBOX_GLOBAL_KEY, TITLE_CE_SIG_KEY, hashlib.sha1).digest()[:16]
    want = hmac.new(auth, buf[:SIG_OFF], hashlib.sha1).digest()[:20]
    return hmac.compare_digest(want, buf[SIG_OFF:SIG_OFF + 20])


def _name(buf: bytes) -> str:
    return buf[NAME_OFF:NAME_OFF + NAME_LEN].decode("utf-16-le", "ignore").split("\x00")[0]


def _snapshot(overlay: str) -> str:
    # reflink CoW clone IN THE OVERLAY'S DIR (the backing ref is a bare basename
    # that must still resolve) so we read a LIVE box without touching its lock.
    d = os.path.dirname(os.path.abspath(overlay))
    dest = os.path.join(d, f".xcarto-gtsnap-{os.getpid()}-{int(time.time()*1000)}.qcow2")
    subprocess.run(["cp", "--reflink=auto", overlay, dest], check=True, capture_output=True)
    return dest


@contextlib.contextmanager
def _mounted(overlay: str, drive: str = "e"):
    from pyfatx import Fatx  # raises ImportError → caller fails soft
    workdir = tempfile.mkdtemp(prefix="xcarto-fatx-")
    raw = os.path.join(workdir, "disk.raw")
    snap = proc = None
    try:
        snap = _snapshot(overlay)
        open(raw, "wb").close()
        proc = subprocess.Popen(
            ["qemu-storage-daemon",
             "--blockdev", f"driver=qcow2,node-name=trdisk,file.driver=file,file.filename={snap}",
             "--export", f"type=fuse,id=trexp,node-name=trdisk,mountpoint={raw},writable=on,allow-other=off"],
            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
        deadline = time.time() + 10
        while time.time() < deadline:
            if proc.poll() is not None:
                err = (proc.stderr.read() or b"").decode("replace").strip()
                raise RuntimeError(f"qemu-storage-daemon exited: {err}")
            try:
                if os.stat(raw).st_size > 0:
                    break
            except FileNotFoundError:
                pass
            time.sleep(0.05)
        else:
            raise RuntimeError("FUSE export not ready")
        yield Fatx(raw, drive=drive)
    finally:
        subprocess.run(["fusermount3", "-u", raw], capture_output=True, check=False)
        if proc is not None:
            proc.kill()
            with contextlib.suppress(Exception):
                proc.wait(timeout=5)
        shutil.rmtree(workdir, ignore_errors=True)
        if snap:
            with contextlib.suppress(FileNotFoundError):
                os.remove(snap)


def enumerate_names(overlay: str, title: str) -> list:
    base = f"/UDATA/{title}"
    names = []
    with _mounted(overlay) as fs:
        # top-level walk of UDATA/<title> → the per-variant save dirs
        dirs = []
        for d, subdirs, _files in fs.walk(base):
            if d.rstrip("/").lower() == base.lower():
                dirs = list(subdirs)
                break
        # FATX directory-entry order == the live SELECT GAMETYPE carousel order
        # (RUNTIME-VALIDATED: the item-list custom sequence [BALL 5M 10S, BALL 5M
        # 7S, CTF 3C 10S, …] matches the on-disk walk order, NOT an alphabetical
        # sort). Do NOT sort — the pack's on-disk order is the menu order.
        for name in dirs:
            try:
                buf = bytes(fs.read(f"{base}/{name}/blam.lst"))
            except Exception:
                continue
            if len(buf) < SIG_OFF + 20 or not _digest_ok(buf):
                continue  # foreign / corrupt / unsigned → never emit
            nm = _name(buf)
            if nm:
                names.append(nm)
    return names


def main() -> int:
    if len(sys.argv) != 3:
        print(json.dumps({"error": "usage: fatx_gametypes.py <overlay> <titleID>"}))
        return 2
    overlay, title = sys.argv[1], sys.argv[2]
    try:
        print(json.dumps({"names": enumerate_names(overlay, title)}))
        return 0
    except Exception as e:  # ImportError (no pyfatx), storage-daemon, FATX, …
        print(json.dumps({"error": str(e)}))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
