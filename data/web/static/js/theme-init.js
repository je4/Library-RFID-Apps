/**
 * Early Theme & Mode Initializer for RFID Desk (FOUC Prevention).
 * Executes synchronously in the <head> before rendering to prevent layout or color flashes.
 */
(function() {
    var DARK_THEMES = ['dark', 'synthwave', 'dracula', 'halloween', 'forest', 'night', 'luxury', 'business', 'coffee', 'dim', 'sunset', 'black', 'abyss'];

    function getCookie(name) {
        var match = document.cookie.match(new RegExp('(?:^|;\\s*)' + name + '=([^;]+)'));
        return match ? decodeURIComponent(match[1]) : null;
    }

    function getStorage(name) {
        try { return localStorage.getItem(name); } catch(e) { return null; }
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

        var activeTheme = 'corporate';
        if (mode === 'auto') {
            activeTheme = isDark ? themeDark : themeLight;
        } else if (isDark) {
            activeTheme = (directTheme && DARK_THEMES.indexOf(directTheme) !== -1) ? directTheme : themeDark;
        } else {
            activeTheme = (directTheme && DARK_THEMES.indexOf(directTheme) === -1) ? directTheme : themeLight;
        }

        if (!activeTheme) {
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
