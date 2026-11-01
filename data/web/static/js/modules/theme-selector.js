/**
 * DaisyUI Theme Controller Module for RFID Desk
 * Implements DaisyUI theme-controller component for seamless CSS-driven
 * theme switching with Cookie / localStorage persistence and visual indicator updates.
 */

export const THEMES = [
    // --- Mediathek Custom Themes ---
    { id: 'mediathek-nord', name: 'Mediathek Nord', type: 'light' },

    // --- Light Themes ---
    { id: 'light',     name: 'Light',     type: 'light' },
    { id: 'corporate', name: 'Corporate', type: 'light' },
    { id: 'emerald',   name: 'Emerald',   type: 'light' },
    { id: 'nord',      name: 'Nord',      type: 'light' },
    { id: 'cupcake',   name: 'Cupcake',   type: 'light' },
    { id: 'bumblebee', name: 'Bumblebee', type: 'light' },
    { id: 'retro',     name: 'Retro',     type: 'light' },
    { id: 'garden',    name: 'Garden',    type: 'light' },
    { id: 'aqua',      name: 'Aqua',      type: 'light' },
    { id: 'lofi',      name: 'Lofi',      type: 'light' },
    { id: 'pastel',    name: 'Pastel',    type: 'light' },
    { id: 'fantasy',   name: 'Fantasy',   type: 'light' },
    { id: 'wireframe', name: 'Wireframe', type: 'light' },
    { id: 'cmyk',      name: 'CMYK',      type: 'light' },
    { id: 'autumn',    name: 'Autumn',    type: 'light' },
    { id: 'acid',      name: 'Acid',      type: 'light' },
    { id: 'lemonade',  name: 'Lemonade',  type: 'light' },
    { id: 'winter',    name: 'Winter',    type: 'light' },
    { id: 'silk',      name: 'Silk',      type: 'light' },
    { id: 'cyberpunk', name: 'Cyberpunk', type: 'light' },
    { id: 'valentine', name: 'Valentine', type: 'light' },

    // --- Dark Themes ---
    { id: 'dark',      name: 'Dark',      type: 'dark' },
    { id: 'synthwave', name: 'Synthwave', type: 'dark' },
    { id: 'halloween', name: 'Halloween', type: 'dark' },
    { id: 'forest',    name: 'Forest',    type: 'dark' },
    { id: 'black',     name: 'Black',     type: 'dark' },
    { id: 'luxury',    name: 'Luxury',    type: 'dark' },
    { id: 'dracula',   name: 'Dracula',   type: 'dark' },
    { id: 'business',  name: 'Business',  type: 'dark' },
    { id: 'night',     name: 'Night',     type: 'dark' },
    { id: 'coffee',    name: 'Coffee',    type: 'dark' },
    { id: 'dim',       name: 'Dim',       type: 'dark' },
    { id: 'sunset',    name: 'Sunset',    type: 'dark' },
    { id: 'abyss',     name: 'Abyss',     type: 'dark' }
];

const DARK_THEME_IDS = new Set([
    'dark', 'synthwave', 'halloween', 'forest', 'black',
    'luxury', 'dracula', 'business', 'night', 'coffee',
    'dim', 'sunset', 'abyss'
]);

export function getCookie(name) {
    const match = document.cookie.match(new RegExp('(?:^|;\\s*)' + name + '=([^;]+)'));
    return match ? decodeURIComponent(match[1]) : null;
}

export function setCookie(name, value, days = 365) {
    const maxAge = days * 24 * 60 * 60;
    document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=${maxAge}; SameSite=Lax`;
}

export function getStorage(name) {
    try {
        return localStorage.getItem(name);
    } catch (_) {
        return null;
    }
}

export function setStorage(name, value) {
    try {
        localStorage.setItem(name, value);
    } catch (_) {}
}

export function isValidTheme(themeId) {
    if (!themeId) return false;
    return THEMES.some(t => t.id === themeId);
}

export function getStoredTheme() {
    return getCookie('rfid_theme') || getStorage('rfid_theme') || null;
}

/**
 * Applies the given theme to document.documentElement and synchronizes inputs, indicators, and storage.
 */
export function applyTheme(themeId, persist = true) {
    if (!themeId || !isValidTheme(themeId)) {
        themeId = 'corporate';
    }

    // 1. Set data-theme on <html>
    document.documentElement.setAttribute('data-theme', themeId);

    // 2. Set dark/light class on <html> for Tailwind dark mode
    const isDark = DARK_THEME_IDS.has(themeId);
    if (isDark) {
        document.documentElement.classList.add('dark');
        document.documentElement.classList.remove('light');
    } else {
        document.documentElement.classList.add('light');
        document.documentElement.classList.remove('dark');
    }

    // 3. Update header theme badge indicator
    const nameBadge = document.getElementById('theme-name-indicator');
    if (nameBadge) {
        nameBadge.textContent = themeId;
    }

    // 4. Update radio inputs with theme-controller class
    const controllers = document.querySelectorAll('input.theme-controller');
    controllers.forEach(input => {
        const isMatch = (input.value === themeId);
        input.checked = isMatch;

        // Toggle checkmark icon in parent item if present
        const label = input.closest('label');
        if (label) {
            const checkIcon = label.querySelector('.theme-check-icon');
            if (checkIcon) {
                checkIcon.classList.toggle('hidden', !isMatch);
            }
        }
    });

    // 5. Persist theme choice
    if (persist) {
        setCookie('rfid_theme', themeId);
        setStorage('rfid_theme', themeId);
        if (isDark) {
            setCookie('rfid_theme_dark', themeId);
            setStorage('rfid_theme_dark', themeId);
        } else {
            setCookie('rfid_theme_light', themeId);
            setStorage('rfid_theme_light', themeId);
        }
    }

    return themeId;
}

/**
 * Initializes the Theme Controller event listeners and synchronizes with current theme.
 */
export function initThemeSelector() {
    // 1. Determine active theme (from DOM attribute, cookie, or storage)
    const activeTheme = document.documentElement.getAttribute('data-theme') ||
                        getStoredTheme() ||
                        'corporate';

    applyTheme(activeTheme, false);

    // 2. Listen for changes on any theme-controller input
    document.addEventListener('change', (e) => {
        if (e.target && e.target.classList && e.target.classList.contains('theme-controller')) {
            applyTheme(e.target.value, true);
        }
    });

    // 3. Close dropdown when clicking an item on mobile / touch
    const dropdownMenu = document.getElementById('theme-dropdown-menu');
    if (dropdownMenu) {
        dropdownMenu.addEventListener('click', (e) => {
            const label = e.target.closest('label');
            if (label) {
                const radio = label.querySelector('input.theme-controller');
                if (radio && radio.value) {
                    applyTheme(radio.value, true);
                    if (document.activeElement && typeof document.activeElement.blur === 'function') {
                        document.activeElement.blur();
                    }
                }
            }
        });
    }
}
