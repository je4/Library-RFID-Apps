import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

globalThis.fetch = async () => ({ ok: true, json: async () =>
    JSON.parse(readFileSync(new URL('../../locales/de.json', import.meta.url), 'utf8')) });
const { initCatalog } = await import('./catalog.js');

class Element {
    constructor(barcode, tagName = '') {
        this.nodeType = 1;
        this.tagName = tagName;
        this.dataset = barcode ? { catalogBarcode: barcode } : {};
        this.children = [];
        this.text = '';
        this.listeners = {};
    }
    set textContent(value) { this.text = value; this.children = []; }
    get textContent() { return this.text + this.children.map(child => child.textContent).join(''); }
    appendChild(child) { this.children.push(child); }
    replaceChildren() { this.children = []; this.text = ''; }
    hasChildNodes() { return this.children.length > 0; }
    setAttribute(name, value) { this[name] = value; }
    addEventListener(name, callback) { this.listeners[name] = callback; }
    showModal() { this.open = true; }
    remove() { this.removed = true; }
    matches() { return !!this.dataset.catalogBarcode; }
    querySelectorAll() { return this.children.flatMap(child => [...(child.matches() ? [child] : []), ...child.querySelectorAll()]); }
}

test('catalog loads initial and dynamic rows, shares lookups and handles failures safely', async () => {
    const body = new Element();
    const first = new Element('x & y');
    const duplicate = new Element('x & y');
    body.appendChild(first);
    body.appendChild(duplicate);
    const calls = [];
    let observer;
    globalThis.window = { location: { pathname: '/desk-a/loan', origin: 'https://example.test' } };
    globalThis.document = {
        body: new Element(),
        getElementById: id => id === 'loan-table-body' ? body : null,
        createElement: tagName => new Element(undefined, tagName)
    };
    globalThis.MutationObserver = class {
        constructor(callback) { observer = callback; }
        observe() {}
    };
    globalThis.fetch = async url => {
        calls.push(url);
        if (url.searchParams.get('barcode') === 'missing') return { ok: true, json: async () => ({ books: [] }) };
        if (url.searchParams.get('barcode') === 'invalid') return { ok: true, json: async () => ({ books: [{ id: 'oai:alma.41SLSP_FNW:bad&value' }] }) };
        if (url.searchParams.get('barcode') === 'error') return { ok: false, status: 503 };
        return { ok: true, json: async () => ({ sourceRecords: [{ network: '41SLSP_NETWORK', baseUrl: url.searchParams.get('barcode') === 'bad-url' ? 'javascript:alert(1)' : url.searchParams.get('barcode') === 'no-url' ? '' : 'https://catalog.example.test/custom/source?existing=1', institutions: { FHNW: '41SLSP_FNW:VU1', ETH: '41SLSP_ETH:ETH' } }], books: [{
            id: url.searchParams.get('barcode') === 'eth' ? 'oai:alma.41SLSP_ETH:123' : url.searchParams.get('barcode') === 'unknown' ? 'oai:alma.UNKNOWN:123' : 'oai:alma.41SLSP_FNW:990048097120205518',
            titles: [{ title: '<img src=x onerror=alert(1)>', subTitle: 'Untertitel' }],
            personalNames: [{ primary: { namePart: 'Autor' } }],
            publications: [{ place: ['Bern'], publisher: ['Verlag'], dateString: '2024' }],
            holdings: [{ callNumber: 'A 1', library: 'Bibliothek' }]
        }] }) };
    };
    const flush = () => new Promise(resolve => setImmediate(resolve));
    initCatalog();
    await flush();
    assert.equal(calls.length, 1);
    assert.equal(calls[0].pathname, '/desk-a/catalog');
    assert.equal(calls[0].searchParams.get('barcode'), 'x & y');
    const book = first.children[0];
    assert.equal(book.children[0].textContent, '<img src=x onerror=alert(1)>');
    const link = book.children[1];
    const source = new URL(link.href);
    assert.equal(source.origin + source.pathname, 'https://catalog.example.test/custom/source');
    assert.equal(source.searchParams.get('existing'), '1');
    assert.equal(source.searchParams.get('vid'), '41SLSP_FNW:VU1');
    assert.equal(source.searchParams.get('docId'), 'alma990048097120205518');
    assert.equal(source.searchParams.get('recordOwner'), '41SLSP_NETWORK');
    let prevented = false;
    link.listeners.click({ button: 0, preventDefault: () => { prevented = true; } });
    assert.equal(prevented, true);
    const dialog = document.body.children[0];
    assert.equal(dialog.open, true);
    assert.equal(dialog.children[0].children[1].src, link.href);
    assert.equal(dialog.children[0].children[2].href, link.href);
    assert.equal(dialog.children[1].tagName, 'form');
    assert.equal(dialog.children[1].className, 'modal-backdrop');
    assert.equal(dialog.children[1].method, 'dialog');
    assert.equal(dialog.children[1].children[0].tagName, 'button');
    dialog.listeners.close();
    assert.equal(dialog.removed, true);
    const details = book.children[2];
    assert.equal(details.tagName, 'details');
    assert.equal(details.open, undefined);
    assert.equal(details.children[0].tagName, 'summary');
    assert.equal(details.children[0].textContent, 'Weitere Angaben');
    assert.deepEqual(details.children.slice(1).map(row => [row.children[0].tagName, row.children[0].textContent]), [
        ['strong', 'Untertitel: '], ['strong', 'Autor: '], ['strong', 'Publikation: '], ['strong', 'Signatur: ']
    ]);
    assert.match(details.textContent, /Untertitel: Untertitel/);
    assert.match(first.textContent, /Autor: Autor/);
    assert.match(first.textContent, /Publikation: Bern, Verlag, 2024/);
    assert.match(first.textContent, /Signatur: A 1 – Bibliothek/);
    assert.equal(first.textContent, duplicate.textContent);

    const eth = new Element('eth');
    observer([{ addedNodes: [eth] }]);
    await flush();
    assert.equal(new URL(eth.children[0].children[1].href).searchParams.get('vid'), '41SLSP_ETH:ETH');
    const unknown = new Element('unknown');
    observer([{ addedNodes: [unknown] }]);
    await flush();
    assert.equal(unknown.children[0].children[1].tagName, 'details');

    for (const barcode of ['bad-url', 'no-url']) {
        const row = new Element(barcode);
        observer([{ addedNodes: [row] }]);
        await flush();
        assert.equal(row.children[0].children[1].tagName, 'details');
    }

    const invalid = new Element('invalid');
    observer([{ addedNodes: [invalid] }]);
    await flush();
    assert.equal(invalid.children[0].children.length, 1);
    const missing = new Element('missing');
    observer([{ addedNodes: [missing] }]);
    await flush();
    assert.equal(missing.textContent, 'Kein Katalogeintrag gefunden');
    const error = new Element('error');
    observer([{ addedNodes: [error] }]);
    await flush();
    assert.equal(error.textContent, 'Katalog nicht verfügbar');
    const disabled = new Element('another');
    observer([{ addedNodes: [disabled] }]);
    await flush();
    assert.equal(disabled.textContent, 'Katalog nicht verfügbar');
    assert.equal(calls.length, 8);
});