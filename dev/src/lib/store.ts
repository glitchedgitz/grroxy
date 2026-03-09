import { writable } from 'svelte/store';

export const baseUrl = writable('http://127.0.0.1:8090');
export const authToken = writable('');
