// Minimal VNC (RFB 3.8) keyboard-only client.
// Connects to a jlesage/firefox container's WebSocket VNC proxy at /websockify,
// performs the handshake, and sends KeyEvent messages. Never requests framebuffer
// data, so zero display bandwidth overhead.

export const KEYSYM: Record<string, number> = {
	// Face buttons
	a: 0x0061,
	b: 0x0062,
	x: 0x0078,
	y: 0x0079,
	// D-pad
	Up: 0xff52,
	Down: 0xff54,
	Left: 0xff51,
	Right: 0xff53,
	// Start / Back / Guide
	Return: 0xff0d,
	BackSpace: 0xff08,
	'5': 0x0035,
	// Bumpers, L3, R3
	'1': 0x0031,
	'2': 0x0032,
	'3': 0x0033,
	'4': 0x0034,
	// Triggers
	w: 0x0077,
	o: 0x006f,
	// Left stick (ESDF)
	e: 0x0065,
	s: 0x0073,
	d: 0x0064,
	f: 0x0066,
	// Right stick (IJKL)
	i: 0x0069,
	j: 0x006a,
	k: 0x006b,
	l: 0x006c,
	// Modifier + letter for xemu's Ctrl+R reset chord
	Control_L: 0xffe3,
	r: 0x0072,
	// Function keys (snapshot save/load and reset shortcuts)
	F1: 0xffbe,
	F2: 0xffbf,
	F3: 0xffc0,
	F4: 0xffc1,
	F5: 0xffc2,
	F6: 0xffc3,
	F7: 0xffc4,
	F8: 0xffc5,
	F9: 0xffc6,
	F10: 0xffc7,
	F11: 0xffc8,
	F12: 0xffc9
};

const enum State {
	Version,
	Security,
	SecurityResult,
	ServerInit,
	Ready
}

export class VNCKeyboard {
	private ws: WebSocket | null = null;
	private state = State.Version;
	private url: string;
	private intentionalClose = false;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private onStatus: ((connected: boolean) => void) | null;

	constructor(url: string, onStatus?: (connected: boolean) => void) {
		this.url = url;
		this.onStatus = onStatus ?? null;
	}

	connect() {
		this.intentionalClose = false;
		this.state = State.Version;
		try {
			this.ws = new WebSocket(this.url, ['binary']);
		} catch {
			this.scheduleReconnect();
			return;
		}
		this.ws.binaryType = 'arraybuffer';
		this.ws.onmessage = (e) => this.onMessage(e.data as ArrayBuffer);
		this.ws.onclose = () => this.onClose();
		this.ws.onerror = () => {}; // onclose fires after onerror
	}

	disconnect() {
		this.intentionalClose = true;
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		this.ws?.close();
		this.ws = null;
		this.onStatus?.(false);
	}

	sendKey(keysym: number, down: boolean) {
		if (!this.ws || this.state !== State.Ready) return;
		const buf = new ArrayBuffer(8);
		const v = new DataView(buf);
		v.setUint8(0, 4); // KeyEvent
		v.setUint8(1, down ? 1 : 0);
		// bytes 2-3: padding (zero)
		v.setUint32(4, keysym);
		this.ws.send(buf);
	}

	get connected(): boolean {
		return this.state === State.Ready && this.ws?.readyState === WebSocket.OPEN;
	}

	private onMessage(data: ArrayBuffer) {
		const view = new DataView(data);
		switch (this.state) {
			case State.Version:
				// Server: "RFB 003.008\n" (12 bytes). Reply with same.
				this.ws!.send(new TextEncoder().encode('RFB 003.008\n'));
				this.state = State.Security;
				break;

			case State.Security: {
				// Server: count (1 byte) + count security types.
				const count = view.getUint8(0);
				let hasNone = false;
				for (let i = 0; i < count; i++) {
					if (view.getUint8(1 + i) === 1) hasNone = true;
				}
				if (!hasNone) {
					console.error('[vnc-keyboard] server requires auth, cannot connect');
					this.ws!.close();
					return;
				}
				this.ws!.send(new Uint8Array([1])); // select None
				this.state = State.SecurityResult;
				break;
			}

			case State.SecurityResult:
				// Server: 4 bytes, 0 = OK.
				if (view.getUint32(0) !== 0) {
					console.error('[vnc-keyboard] security handshake failed');
					this.ws!.close();
					return;
				}
				// ClientInit: shared-flag = 1 (coexist with noVNC display connection)
				this.ws!.send(new Uint8Array([1]));
				this.state = State.ServerInit;
				break;

			case State.ServerInit: {
				// Advertise DesktopSize so Xvnc doesn't kick us when the iframe's
				// noVNC client renegotiates the framebuffer size.
				const encs = [-223, -308, -224]; // DesktopSize, ExtendedDesktopSize, LastRect
				const buf = new ArrayBuffer(4 + 4 * encs.length);
				const v = new DataView(buf);
				v.setUint8(0, 2); // SetEncodings
				v.setUint8(1, 0); // padding
				v.setUint16(2, encs.length);
				for (let i = 0; i < encs.length; i++) {
					v.setInt32(4 + i * 4, encs[i]);
				}
				this.ws!.send(buf);
				this.state = State.Ready;
				this.onStatus?.(true);
				break;
			}

			case State.Ready:
				// Ignore any unsolicited messages (Bell, ServerCutText, etc.)
				break;
		}
	}

	private onClose() {
		this.ws = null;
		const wasReady = this.state === State.Ready;
		this.state = State.Version;
		if (wasReady) this.onStatus?.(false);
		if (!this.intentionalClose) this.scheduleReconnect();
	}

	private scheduleReconnect() {
		if (this.reconnectTimer) return;
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			if (!this.intentionalClose) this.connect();
		}, 1000);
	}
}
