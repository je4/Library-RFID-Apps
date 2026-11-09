const language = (globalThis.document?.documentElement?.lang || '').toLowerCase().split('-')[0];
const selectedLanguage = ['de', 'en', 'fr', 'it'].includes(language) ? language : 'de';

async function loadCatalog(lang) {
    try {
        const response = await fetch(new URL(`../../locales/${lang}.json`, import.meta.url));
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        return await response.json();
    } catch (error) {
        console.error(`Unable to load ${lang} translations`, error);
        return {};
    }
}

const de = await loadCatalog('de');
const selected = selectedLanguage === 'de' ? de : await loadCatalog(selectedLanguage);

export function t(key, params = {}) {
    const text = Object.hasOwn(selected, key) ? selected[key] : Object.hasOwn(de, key) ? de[key] : key;
    return text.replace(/\{(\w+)\}/g, (match, name) =>
        Object.hasOwn(params, name) ? String(params[name]) : match);
}