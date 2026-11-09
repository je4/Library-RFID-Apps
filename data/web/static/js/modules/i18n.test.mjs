import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const catalogs = Object.fromEntries(['de', 'en', 'fr', 'it'].map(lang => [lang,
    JSON.parse(readFileSync(new URL(`../../locales/${lang}.json`, import.meta.url), 'utf8'))]));

async function load(lang, failed = []) {
    globalThis.document = { documentElement: { lang } };
    globalThis.fetch = async url => {
        const language = url.pathname.match(/\/([a-z]+)\.json$/)[1];
        if (failed.includes(language)) return { ok: false, status: 500 };
        return { ok: true, json: async () => catalogs[language] };
    };
    return (await import(`./i18n.js?test=${encodeURIComponent(lang)}-${failed.join('-')}`)).t;
}

test('shared catalogs supply translations, interpolate safely and fall back', async () => {
    const de = await load('de');
    assert.equal(de('encode.partProgress', { part: 2, total: 3 }), 'Teil 2 von 3');
    assert.equal(de('encode.writeMessage', { barcode: '{uid}', uid: '<tag>' }),
        'Schreibe Barcode {uid} auf Transponder <tag>... Bitte nicht entfernen.');
    assert.equal(de('encode.partProgress', { part: 1 }), 'Teil 1 von {total}');
    assert.equal(de('missing.key'), 'missing.key');
    assert.equal(de('toString'), 'toString');
    for (const [lang, expected] of [['en-US', 'Close'], ['FR-ch', 'Fermer'], ['it', 'Chiudi'], ['es', 'Schliessen']]) {
        assert.equal((await load(lang))('catalog.close'), expected);
    }
});

test('catalog keys and named placeholders match across languages', () => {
    const german = catalogs.de;
    for (const lang of ['en', 'fr', 'it']) {
        assert.deepEqual(Object.keys(catalogs[lang]).sort(), Object.keys(german).sort());
        for (const key of Object.keys(german)) {
            assert.deepEqual(catalogs[lang][key].match(/\{\w+\}/g)?.sort() ?? [],
                german[key].match(/\{\w+\}/g)?.sort() ?? [], `${lang}: ${key}`);
        }
    }
});

test('failed selected catalog falls back to German', async () => {
    const previousError = console.error;
    console.error = () => {};
    try {
        assert.equal((await load('en', ['en']))('catalog.close'), 'Schliessen');
    } finally {
        console.error = previousError;
    }
});