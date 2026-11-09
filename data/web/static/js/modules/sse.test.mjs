import { test } from 'node:test';
import assert from 'node:assert/strict';

globalThis.document = {
    documentElement: { lang: 'de' },
    getElementById: () => null,
};
globalThis.fetch = async () => ({ ok: true, json: async () => ({}) });

const { initSSE } = await import('./sse.js');

test('initial status reflects an SSE connection that opened before initialization', () => {
    const text = { textContent: '' };
    document.getElementById = id => id === 'sse-status-text' ? text : null;
    document.addEventListener = () => {};
    document.querySelectorAll = () => [];
    document.body = { _htmx_sse: { sseSource: { readyState: 1 } }, addEventListener() {} };

    initSSE();
    assert.equal(text.textContent, 'sse.connected');
});

test('initial status reflects a previously failed connection', () => {
    const text = { textContent: '' };
    document.getElementById = id => id === 'sse-status-text' ? text : null;
    document.body._htmx_sse.sseSource.readyState = 2;

    initSSE();
    assert.equal(text.textContent, 'sse.reconnecting');
});