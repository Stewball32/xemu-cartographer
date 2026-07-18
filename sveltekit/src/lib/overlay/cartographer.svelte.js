// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked
// Runes-based WebSocket feed for xemu-cartographer (.svelte.js so $state works).
// Message contract: see README.md.
export class CartographerFeed {
	connected = $state(false);
	players = $state([]);
	match = $state({}); // { gametype, map, clock, phase, killLimit, matchTime, teams: [{id,name,score}] | null }

	#ws;
	#timer;
	#dead = false;
	#delay = 500;

	constructor(url, { nameOverrides = {} } = {}) {
		this.url = url;
		this.nameOverrides = nameOverrides;
		this.#connect();
	}

	#connect() {
		this.#ws = new WebSocket(this.url);
		this.#ws.onopen = () => {
			this.#delay = 500;
		};
		this.#ws.onmessage = (e) => {
			try {
				const msg = JSON.parse(e.data);
				if (msg.type !== 'state') return;
				this.players = (msg.players ?? []).map((p) => ({
					...p,
					name: this.nameOverrides[p.name] ?? p.name
				}));
				this.match = msg.match ?? {};
				this.connected = true;
			} catch {
				/* ignore malformed frames */
			}
		};
		this.#ws.onclose = () => {
			this.connected = false;
			if (!this.#dead) {
				this.#timer = setTimeout(() => this.#connect(), this.#delay);
				this.#delay = Math.min(this.#delay * 2, 8000);
			}
		};
		this.#ws.onerror = () => this.#ws.close();
	}

	destroy() {
		this.#dead = true;
		clearTimeout(this.#timer);
		this.#ws?.close();
	}
}
