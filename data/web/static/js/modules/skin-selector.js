/**
 * Skin & Theme Selector Module for RFID Desk
 * Handles theme switching, dual data-skin and DaisyUI data-theme DOM updates,
 * and cookie/localStorage persistence.
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

export function applyTheme(skin) {
    const isDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    const effectiveSkin = skin || 'default';

    // 1. Set data-skin
    if (effectiveSkin && effectiveSkin !== 'default') {
        document.documentElement.setAttribute('data-skin', effectiveSkin);
    } else {
        document.documentElement.removeAttribute('data-skin');
    }

    // 2. Set DaisyUI data-theme
    const daisyTheme = resolveDaisyTheme(effectiveSkin, isDark);
    document.documentElement.setAttribute('data-theme', daisyTheme);
}

export function initSkinSelector() {
    const skinSelect = document.getElementById('skin-select');

    // Synchronize select value with currently applied skin or default
    const currentSkin = document.documentElement.getAttribute('data-skin') || 'default';
    if (skinSelect) {
        skinSelect.value = currentSkin;
    }

    // Ensure initial theme sync
    applyTheme(currentSkin);

    if (skinSelect) {
        skinSelect.addEventListener('change', (e) => {
            const selectedSkin = e.target.value;

            // 1. Immediately apply to DOM
            applyTheme(selectedSkin);

            // 2. Persist in Cookie (1 year duration, path=/, SameSite=Lax)
            document.cookie = `rfid_skin=${encodeURIComponent(selectedSkin)}; path=/; max-age=31536000; SameSite=Lax`;

            // 3. Fallback to localStorage
            try {
                localStorage.setItem('rfid_skin', selectedSkin);
            } catch (_) {}
        });
    }

    // React to system color scheme changes
    if (window.matchMedia) {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
            const activeSkin = document.documentElement.getAttribute('data-skin') || 'default';
            applyTheme(activeSkin);
        });
    }
}
