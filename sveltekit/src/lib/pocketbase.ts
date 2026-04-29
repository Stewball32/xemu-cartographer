import PocketBase from 'pocketbase';
import type { TypedPocketBase } from '$lib/types/pocketbase-types';
import { apiBaseURL } from '$lib/utils/api-base';

const pb = new PocketBase(apiBaseURL() || undefined) as TypedPocketBase;

export default pb;
