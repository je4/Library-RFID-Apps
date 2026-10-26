/**
 * Early Theme & Mode Initializer for RFID Desk (FOUC Prevention).
 * Executes synchronously in the <head> before rendering to prevent layout or color flashes.
 */
(function() {
    var DARK_THEMES = ['dark', 'synthwave', 'dracula', 'halloween', 'forest', 'night', 'luxury', 'business', 'coffee', 'dim', 'sunset', 'black', 'abyss'];
    var LIGHT_THEMES = ['corporate', 'light', 'emerald', 'nord', 'cupcake', 'retro', 'garden', 'aqua', 'lofi', 'pastel', 'fantasy', 'winter', 'autumn', 'acid', 'lemonade', 'cyberpunk', 'valentine', 'wireframe', 'cmyk', 'silk'];

    function getCookie(name) {
        var match = document.cookie.match(new RegExp('(?:^|;\\s*)' + name + '=([^;]+)'));
        return match ? decodeURIComponent(match[1]) : null;
    }

    function getStorage(name) {
        try { return localStorage.getItem(name); } catch(e) { return null; }
    }

    function isValidTheme(theme, isDark) {
        if (!theme) return false;
        return isDark ? (DARK_THEMES.indexOf(theme) !== -1) : (LIGHT_THEMES.indexOf(theme) !== -1);
    }

    function syncTheme() {
        var mode = getCookie('rfid_theme_mode') || getStorage('rfid_theme_mode') || 'auto';
        var themeLight = getCookie('rfid_theme_light') || getStorage('rfid_theme_light') || 'corporate';
        var themeDark = getCookie('rfid_theme_dark') || getStorage('rfid_theme_dark') || 'dark';
        var directTheme = getCookie('rfid_theme') || getStorage('rfid_theme');

        var isDark = false;
        if (mode === 'darkmode' || mode === 'dark') {
            isDark = true;
        } else if (mode === 'lightmode' || mode === 'light') {
            isDark = false;
        } else {
            isDark = Boolean(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
        }

        if (isDark) {
            document.documentElement.classList.add('dark');
            document.documentElement.classList.remove('light');
        } else {
            document.documentElement.classList.add('light');
            document.documentElement.classList.remove('dark');
        }

        if (!isValidTheme(themeLight, false)) themeLight = 'corporate';
        if (!isValidTheme(themeDark, true)) themeDark = 'dark';

        var activeTheme = 'corporate';
        if (mode === 'auto') {
            activeTheme = isDark ? themeDark : themeLight;
        } else if (isDark) {
            activeTheme = isValidTheme(directTheme, true) ? directTheme : themeDark;
        } else {
            activeTheme = isValidTheme(directTheme, false) ? directTheme : themeLight;
        }

        if (!activeTheme || !isValidTheme(activeTheme, isDark)) {
            activeTheme = isDark ? 'dark' : 'corporate';
        }

        document.documentElement.setAttribute('data-theme', activeTheme);
    }

    syncTheme();

    if (window.matchMedia) {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function() {
            var mode = getCookie('rfid_theme_mode') || getStorage('rfid_theme_mode') || 'auto';
            if (mode === 'auto') {
                syncTheme();
            }
        });
    }
})();
