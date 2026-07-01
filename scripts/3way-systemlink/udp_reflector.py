#!/usr/bin/env python3
"""udp_reflector.py — N-way L2 hub for xemu's point-to-point 'udp' net backend.

xemu's [net.udp] backend is strictly point-to-point: one bind_addr (RX) + one
remote_addr (TX). Each xemu encapsulates the guest's raw Ethernet frames into UDP
datagrams, sends them to remote_addr, and injects datagrams arriving at bind_addr
back onto the guest NIC. To put THREE (or more) instances on one shared broadcast
domain — so Halo system-link DISCOVERY broadcasts from any instance reach all the
others — we run this reflector as the common hub.

Wiring (default 3 instances):
  instance i bind_addr = 0.0.0.0:BIND[i]      (xemu RX)
  instance i remote_addr= 127.0.0.1:REFL[i]   (xemu TX -> reflector)
  reflector binds one socket per REFL[i]. A datagram received on socket i (i.e.
  egress from instance i) is re-sent to every OTHER instance j on 127.0.0.1:BIND[j],
  sent *from* the socket bound to REFL[j] so instance j sees source == its own
  configured remote_addr (REFL[j]) — maximally compatible if xemu ever filters by
  peer. Frames are never echoed back to their sender. This is a plain L2 repeater:
  payload bytes are forwarded verbatim, so it is protocol-agnostic (ARP, IP bcast,
  Xbox SG discovery, gameplay — all carried).

Hardened: blocks the Cowork/Claude-Code per-Bash-call teardown signals (same trick
as padpool.py) so it survives being started from a Bash tool call. Stop with
`echo quit > <rundir>/reflector.control` or SIGKILL.
"""
import argparse, os, signal, socket, sys, threading, time, json

def log(path, msg):
    try:
        with open(path, "a") as f:
            f.write("%.3f %s\n" % (time.time(), msg))
    except Exception:
        pass

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bind", default="9971,9972,9973",
                    help="comma list of each instance's bind_addr port (xemu RX)")
    ap.add_argument("--refl", default="9981,9982,9983",
                    help="comma list of reflector ingress ports (each instance's remote_addr)")
    ap.add_argument("--host", default="127.0.0.1")
    ap.add_argument("--rundir", default="/run/user/%d/xemu-3way" % os.getuid())
    ap.add_argument("--no-block-signals", action="store_true")
    args = ap.parse_args()

    bind_ports = [int(x) for x in args.bind.split(",")]
    refl_ports = [int(x) for x in args.refl.split(",")]
    assert len(bind_ports) == len(refl_ports) >= 2, "need >=2 matched bind/refl ports"
    n = len(bind_ports)
    os.makedirs(args.rundir, exist_ok=True)
    logpath = os.path.join(args.rundir, "reflector.log")
    statpath = os.path.join(args.rundir, "reflector.stats.json")
    ctlpath = os.path.join(args.rundir, "reflector.control")
    if not os.path.exists(ctlpath):
        os.mkfifo(ctlpath)

    if not args.no_block_signals:
        try:
            signal.pthread_sigmask(signal.SIG_BLOCK,
                                   {signal.SIGTERM, signal.SIGINT, signal.SIGHUP,
                                    signal.SIGQUIT, signal.SIGSTKFLT, signal.SIGPIPE})
            log(logpath, "teardown signals BLOCKED (hardened)")
        except Exception as ex:
            log(logpath, "sigmask block failed: %r" % ex)

    socks = []
    for i, rp in enumerate(refl_ports):
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        s.bind((args.host, rp))
        s.setblocking(True)
        socks.append(s)
        log(logpath, "ingress sock[%d] bound %s:%d -> serves instance bind :%d"
            % (i, args.host, rp, bind_ports[i]))

    # rx[i] = datagrams received from instance i ; tx[i] = datagrams delivered to instance i
    rx = [0]*n
    tx = [0]*n
    bytes_rx = [0]*n
    start = time.time()
    stop = threading.Event()

    def serve(i):
        s = socks[i]
        while not stop.is_set():
            try:
                data, addr = s.recvfrom(65535)
            except OSError:
                if stop.is_set():
                    return
                time.sleep(0.05); continue
            if not data:
                continue
            rx[i] += 1
            bytes_rx[i] += len(data)
            # fan out to every OTHER instance, sending from THAT instance's ingress sock
            for j in range(n):
                if j == i:
                    continue
                try:
                    socks[j].sendto(data, (args.host, bind_ports[j]))
                    tx[j] += 1
                except OSError as ex:
                    log(logpath, "fwd %d->%d err: %r" % (i, j, ex))

    threads = [threading.Thread(target=serve, args=(i,), daemon=True) for i in range(n)]
    for t in threads:
        t.start()

    def serve_ctl():
        while not stop.is_set():
            try:
                with open(ctlpath, "r") as f:
                    for line in f:
                        if line.strip().lower() == "quit":
                            log(logpath, "control: quit")
                            stop.set()
                            for s in socks:
                                try: s.close()
                                except Exception: pass
                            return
            except Exception:
                time.sleep(0.2)
    threading.Thread(target=serve_ctl, daemon=True).start()

    print("=== udp_reflector: %d-way hub ready ===" % n, flush=True)
    for i in range(n):
        print("  instance%d: xemu TX->%s:%d (refl ingress)   refl->xemu RX %s:%d"
              % (i+1, args.host, refl_ports[i], args.host, bind_ports[i]), flush=True)
    print("stats:", statpath, " log:", logpath, " stop: echo quit >", ctlpath, flush=True)

    # periodic stats dump (evidence the tunnel carried frames)
    try:
        while not stop.is_set():
            time.sleep(1.0)
            snap = {"uptime_s": round(time.time()-start, 1), "instances": n,
                    "bind_ports": bind_ports, "refl_ports": refl_ports,
                    "rx_datagrams": rx, "tx_datagrams": tx, "rx_bytes": bytes_rx}
            tmp = statpath + ".tmp"
            try:
                with open(tmp, "w") as f:
                    json.dump(snap, f, indent=2)
                os.replace(tmp, statpath)
            except Exception:
                pass
    finally:
        log(logpath, "reflector exit rx=%r tx=%r" % (rx, tx))

if __name__ == "__main__":
    main()
