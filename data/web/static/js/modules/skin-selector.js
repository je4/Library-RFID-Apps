/**
 * Skin & Theme Mode Selector Module for RFID Desk
 * Handles skin and light/dark mode switching, dual data-skin and DaisyUI data-theme DOM updates,
 * and cookie/localStorage persistence for rfid_skin and rfid_theme_mode.
 */

export const THEME_MAP = {
    'default':   { light: 'corporate', dark: 'dark' },
    'emerald':   { light: 'emerald',   dark: 'emerald' },
    'nord':      { light: 'nord',      dark: 'nord' },
    'bubblegum': { light: 'cupcake',   dark: 'synthwave' }
};

export function resolveDaisyTheme(skin, isDark) {
    const mapping = THEME_MAP[skin] || THEME_MAP['default'];
    return isDark ? mapping.dark : mapping.light;
}

export function getStoredMode() {
    const match = document.cookie.match(/(?:^|;\s*)rfid_theme_mode=([^;]+)/);
    if (match) return decodeURIComponent(match[1]);
    try {
        const stored = localStorage.getItem('rfid_theme_mode');
        if (stored) return stored;
    } catch (_) {}
    return 'auto';
}

export function getStoredSkin() {
    const match = document.cookie.match(/(?:^|;\s*)rfid_skin=([^;]+)/);
    if (match) return decodeURIComponent(match[1]);
    try {
        const stored = localStorage.getItem('rfid_skin');
        if (stored) return stored;
    } catch (_) {}
    return 'default';
}

export function applyTheme(skin, mode) {
    const effectiveSkin = skin || document.documentElement.getAttribute('data-skin') || getStoredSkin();
    const effectiveMode = mode || getStoredMode();

    let isDark = false;
    if (effectiveMode === 'darkmode' || effectiveMode === 'dark') {
        isDark = true;
    } else if (effectiveMode === 'lightmode' || effectiveMode === 'light') {
        isDark = false;
    } else {
        isDark = Boolean(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
    }

    // 1. Update light/dark classes on <html>
    if (isDark) {
        document.documentElement.classList.add('dark');
        document.documentElement.classList.remove('light');
    } else {
        document.documentElement.classList.add('light');
        document.documentElement.classList.remove('dark');
    }

    // 2. Set data-skin
    if (effectiveSkin && effectiveSkin !== 'default') {
        document.documentElement.setAttribute('data-skin', effectiveSkin);
    } else {
        document.documentElement.removeAttribute('data-skin');
    }

    // 3. Set DaisyUI data-theme
    const daisyTheme = resolveDaisyTheme(effectiveSkin, isDark);
    document.documentElement.setAttribute('data-theme', daisyTheme);
}

export function initSkinSelector() {
    const skinSelect = document.getElementById('skin-select');
    const modeSelect = document.getElementById('theme-mode-select');

    const currentSkin = document.documentElement.getAttribute('data-skin') || getStoredSkin();
    const currentMode = getStoredMode();

    // Synchronize select values with currently applied values
    if (skinSelect) {
        skinSelect.value = currentSkin;
    }
    if (modeSelect) {
        modeSelect.value = currentMode;
    }

    // Ensure initial theme sync
    applyTheme(currentSkin, currentMode);

    if (skinSelect) {
        skinSelect.addEventListener('change', (e) => {
            const selectedSkin = e.target.value;
            const activeMode = modeSelect ? modeSelect.value : getStoredMode();

            // 1. Immediately apply to DOM
            applyTheme(selectedSkin, activeMode);

            // 2. Persist in Cookie (1 year duration, path=/, SameSite=Lax)
            document.cookie = `rfid_skin=${encodeURIComponent(selectedSkin)}; path=/; max-age=31536000; SameSite=Lax`;

            // 3. Fallback to localStorage
            try {
                localStorage.setItem('rfid_skin', selectedSkin);
            } catch (_) {}
        });
    }

    if (modeSelect) {
        modeSelect.addEventListener('change', (e) => {
            const selectedMode = e.target.value;
            const activeSkin = skinSelect ? skinSelect.value : (document.documentElement.getAttribute('data-skin') || getStoredSkin());

            // 1. Immediately apply to DOM
            applyTheme(activeSkin, selectedMode);

            // 2. Persist in Cookie (1 year duration, path=/, SameSite=Lax)
            document.cookie = `rfid_theme_mode=${encodeURIComponent(selectedMode)}; path=/; max-age=31536000; SameSite=Lax`;

            // 3. Fallback to localStorage
            try {
                localStorage.setItem('rfid_theme_mode', selectedMode);
            } catch (_) {}
        });
    }

    // React to system color scheme changes when in 'auto' mode
    if (window.matchMedia) {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
            const activeMode = modeSelect ? modeSelect.value : getStoredMode();
            if (activeMode === 'auto') {
                const activeSkin = skinSelect ? skinSelect.value : (document.documentElement.getAttribute('data-skin') || getStoredSkin());
                applyTheme(activeSkin, 'auto');
            }
        });
    }
}
