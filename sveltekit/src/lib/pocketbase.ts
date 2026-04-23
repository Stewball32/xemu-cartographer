import PocketBase from 'pocketbase';
import type { TypedPocketBase } from '$lib/types/pocketbase-types';
import { PUBLIC_PB_PORT } from '$env/static/public';
import { dev } from '$app/environment';

const pb = new PocketBase(
	dev ? `http://localhost:${PUBLIC_PB_PORT}` : undefined
) as TypedPocketBase;

export default pb;
