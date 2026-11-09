const cache = new Map();
const CACHE_LIMIT = 300;
const SUCCESS_TTL = 5 * 60 * 1000;
const ERROR_TTL = 30 * 1000;
let unavailableUntil = 0;

function catalogURL(barcode) {
    let path = window.location.pathname.replace(/\/(?:loan|encode|inventory)\/?$/, '/');
    if (!path.endsWith('/')) path += '/';
    const url = new URL(path + 'catalog', window.location.origin);
    url.searchParams.set('barcode', barcode);
    return url;
}

function lookup(barcode) {
    const now = Date.now();
    const cached = cache.get(barcode);
    if (cached && cached.expires > now) return cached.promise;
    if (now < unavailableUntil) return Promise.resolve({ message: 'Katalog nicht verfügbar' });

    const entry = { expires: Infinity };
    entry.promise = fetch(catalogURL(barcode))
        .then(async response => {
            if (!response.ok) {
                if (response.status === 503) unavailableUntil = Date.now() + ERROR_TTL;
                throw new Error('catalog unavailable');
            }
            const data = await response.json();
            if (!Array.isArray(data.books)) throw new Error('invalid catalog response');
            entry.expires = Date.now() + SUCCESS_TTL;
            return { books: data.books, sourceRecords: data.sourceRecords };
        })
        .catch(() => {
            entry.expires = Date.now() + ERROR_TTL;
            return { message: 'Katalog nicht verfügbar' };
        });
    cache.delete(barcode);
    cache.set(barcode, entry);
    if (cache.size > CACHE_LIMIT) cache.delete(cache.keys().next().value);
    return entry.promise;
}

function values(items, mapper) {
    return Array.isArray(items) ? items.flatMap(mapper).filter(Boolean) : [];
}

function sourceRecordURL(id, mappings) {
    const match = typeof id === 'string' && /^oai:alma\.([A-Za-z0-9_]+):(\d+)$/.exec(id);
    if (!match) return null;
    let view, network, baseURL;
    for (const mapping of Array.isArray(mappings) ? mappings : []) {
        for (const candidate of Object.values(mapping?.institutions || {})) {
            if (typeof candidate === 'string' && candidate.split(':')[0] === match[1] && mapping.network) {
                if (view && (view !== candidate || network !== mapping.network || baseURL !== mapping.baseUrl)) return null;
                view = candidate;
                network = mapping.network;
                baseURL = mapping.baseUrl;
            }
        }
    }
    if (!view || typeof baseURL !== 'string' || !baseURL.trim()) return null;
    let url;
    try {
        url = new URL(baseURL);
    } catch {
        return null;
    }
    if (!['http:', 'https:'].includes(url.protocol)) return null;
    url.searchParams.set('vid', view);
    url.searchParams.set('docId', 'alma' + match[2]);
    url.searchParams.set('recordOwner', network);
    return url.href;
}

function openSourceRecord(url) {
    const dialog = document.createElement('dialog');
    dialog.className = 'modal';
    dialog.setAttribute('aria-label', 'Quelldaten');
    const box = document.createElement('div');
    box.className = 'modal-box w-11/12 max-w-5xl';
    const heading = document.createElement('h2');
    heading.textContent = 'Quelldaten';
    const frame = document.createElement('iframe');
    frame.title = 'Quelldaten auf Swisscovery';
    frame.src = url;
    frame.className = 'w-full h-[70vh]';
    const fallback = document.createElement('a');
    fallback.href = url;
    fallback.target = '_blank';
    fallback.rel = 'noopener noreferrer';
    fallback.className = 'link';
    fallback.textContent = 'Quelldaten in neuem Tab öffnen';
    const form = document.createElement('form');
    form.method = 'dialog';
    form.className = 'modal-action';
    const close = document.createElement('button');
    close.textContent = 'Schliessen';
    close.className = 'btn';
    form.appendChild(close);
    box.appendChild(heading);
    box.appendChild(frame);
    box.appendChild(fallback);
    box.appendChild(form);
    dialog.appendChild(box);
    dialog.addEventListener('close', () => dialog.remove());
    document.body.appendChild(dialog);
    dialog.showModal();
}

function render(target, result) {
    target.replaceChildren();
    if (result.message || !result.books.length) {
        target.textContent = result.message || 'Kein Katalogeintrag gefunden';
        return;
    }
    for (const book of result.books) {
        if (!book || typeof book !== 'object') continue;
        const lines = [];
        const titles = values(book.titles, title => title?.title);
        const subtitles = values(book.titles, title => title?.subTitle);
        if (subtitles.length) lines.push(['Untertitel', subtitles.join(' / ')]);
        const names = values([...(book.personalNames || []), ...(book.corporateNames || [])], group =>
            group && typeof group === 'object' ? Object.values(group).map(name => name?.namePart) : []);
        if (names.length) lines.push(['Autor', [...new Set(names)].join(', ')]);
        const publications = values(book.publications, publication => publication && [
            ...(publication.place || []), ...(publication.publisher || []), publication.dateString
        ].filter(Boolean).join(', '));
        if (publications.length) lines.push(['Publikation', publications.join(' / ')]);
        const holdings = values(book.holdings, holding => {
            if (!holding) return [];
            const signatures = values(holding.items, item => item?.itemCallNumber);
            if (holding.callNumber) signatures.push(holding.callNumber);
            return [...new Set(signatures)].map(signature => [signature, holding.library, holding.location].filter(Boolean).join(' – '));
        });
        if (holdings.length) lines.push(['Signatur', holdings.join(' / ')]);
        const detail = document.createElement('div');
        detail.className = 'text-xs font-normal';
        const title = document.createElement('div');
        title.textContent = titles.join(' / ') || 'Titel nicht verfügbar';
        detail.appendChild(title);
        const sourceURL = sourceRecordURL(book.id, result.sourceRecords);
        if (sourceURL) {
            const link = document.createElement('a');
            link.href = sourceURL;
            link.className = 'link';
            link.textContent = 'Quelldaten';
            link.addEventListener('click', event => {
                if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
                event.preventDefault();
                openSourceRecord(sourceURL);
            });
            detail.appendChild(link);
        }
        if (lines.length) {
            const details = document.createElement('details');
            const summary = document.createElement('summary');
            summary.textContent = 'Weitere Angaben';
            details.appendChild(summary);
            for (const [label, value] of lines) {
                const row = document.createElement('div');
                const heading = document.createElement('strong');
                heading.textContent = label + ': ';
                row.appendChild(heading);
                const text = document.createElement('span');
                text.textContent = value;
                row.appendChild(text);
                details.appendChild(row);
            }
            detail.appendChild(details);
        }
        target.appendChild(detail);
    }
    if (!target.hasChildNodes()) target.textContent = 'Kein Katalogeintrag gefunden';
}

function load(target) {
    if (target.dataset.catalogLoaded) return;
    const barcode = target.dataset.catalogBarcode?.trim();
    if (!barcode) return;
    target.dataset.catalogLoaded = 'true';
    target.textContent = 'Katalog wird geladen…';
    lookup(barcode).then(result => render(target, result));
}

export function initCatalog() {
    for (const id of ['tags-table-body', 'loan-table-body', 'inventory-table-body']) {
        const body = document.getElementById(id);
        if (!body) continue;
        body.querySelectorAll('[data-catalog-barcode]').forEach(load);
        new MutationObserver(records => {
            for (const record of records) {
                for (const node of record.addedNodes) {
                    if (node.nodeType !== 1) continue;
                    if (node.matches('[data-catalog-barcode]')) load(node);
                    node.querySelectorAll('[data-catalog-barcode]').forEach(load);
                }
            }
        }).observe(body, { childList: true, subtree: true });
    }
}