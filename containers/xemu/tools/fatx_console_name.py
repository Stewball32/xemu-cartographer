#!/usr/bin/env python3
"""Write E:\\UDATA\\NICKNAME.XBN into a raw Xbox HDD image/device via FATX.

Invoked by the Go podman provisioner (internal/podman/console_name.go) at
container-create time, against a raw block view of the per-instance qcow2
OVERLAY (exposed with qemu-storage-daemon's FUSE export, so writes land in the
overlay layer — never the shared read-only root). All Xbox/FATX format
knowledge lives in Go (buildNicknameXBN); this helper is a thin pyfatx shim that
just writes the supplied bytes to the right path on the E: partition.

Usage:
    fatx_console_name.py <raw-image-or-device> <base64-payload>

The payload is the full, fixed-size (3400-byte) NICKNAME.XBN content. The file
is created when absent (FATX write-CREATE via pyfatx) or overwritten when
present, then truncated to the exact payload length.

Requires: pyfatx (`pip install pyfatx`). Exits non-zero with a clear message on
any failure so the caller can log a warning and continue (the instance still
boots — it just gets the Xbox's random default name).
"""
import base64
import sys

PATH = "/UDATA/NICKNAME.XBN"


def main(argv):
    if len(argv) != 3:
        sys.stderr.write("usage: fatx_console_name.py <raw-image> <base64-payload>\n")
        return 2
    raw, b64 = argv[1], argv[2]
    try:
        payload = base64.b64decode(b64)
    except Exception as e:  # noqa: BLE001
        sys.stderr.write(f"bad base64 payload: {e}\n")
        return 2

    try:
        import pyfatx
    except ImportError:
        sys.stderr.write("pyfatx not installed (pip install pyfatx); skipping console-name write\n")
        return 3

    try:
        fs = pyfatx.Fatx(raw, drive="e")  # standard retail E: offset/size
    except Exception as e:  # noqa: BLE001
        sys.stderr.write(f"open FATX E: on {raw} failed: {e}\n")
        return 4

    try:
        # write() creates the file if absent (falls back to create_dirent),
        # otherwise overwrites from offset 0. Truncate to the exact size so a
        # pre-existing longer file can't leave trailing bytes.
        fs.write(PATH, payload)
        try:
            fs.truncate(PATH, len(payload))
        except Exception:  # noqa: BLE001
            pass  # truncate is best-effort; payload already fully overwrote
        # Read back to confirm.
        rb = fs.read(PATH)
    except Exception as e:  # noqa: BLE001
        sys.stderr.write(f"write {PATH} failed: {e}\n")
        return 5

    if rb[:len(payload)] != payload:
        sys.stderr.write("readback mismatch after write\n")
        return 6
    sys.stderr.write(f"wrote {PATH} ({len(payload)} bytes) OK\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
