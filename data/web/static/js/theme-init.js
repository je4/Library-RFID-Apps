/**
 * Early Theme Initializer for RFID Desk (FOUC Prevention).
 * Executes synchronously in the <head> before rendering to prevent layout or color flashes.
 */
(function() {
    var DARK_THEMES = [
        'mediathek-dark',
        'dark', 'synthwave', 'halloween', 'forest', 'black',
        'luxury', 'dracula', 'business', 'night', 'coffee',
        'dim', 'sunset', 'abyss'
    ];

    function getCookie(name) {
        var match = document.cookie.match(new RegExp('(?:^|;\\s*)' + name + '=([^;]+)'));
        return match ? decodeURIComponent(match[1]) : null;
    }

    function getStorage(name) {
        try { return localStorage.getItem(name); } catch(e) { return null; }
    }

    function syncTheme() {
        var doc = document.documentElement;
        var configLight = doc.getAttribute('data-theme-light');
        var configDark = doc.getAttribute('data-theme-dark');
        var hasExplicitThemes = Boolean(configLight && configDark);

        var mode = getCookie('rfid_theme_mode') || getStorage('rfid_theme_mode') || doc.getAttribute('data-theme-mode') || 'auto';
        var activeTheme = '';

        if (hasExplicitThemes) {
            if (mode === 'light' || mode === 'lightmode') {
                activeTheme = configLight;
            } else if (mode === 'dark' || mode === 'darkmode') {
                activeTheme = configDark;
            } else {
                var isDark = Boolean(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
                activeTheme = isDark ? configDark : configLight;
            }
            doc.setAttribute('data-theme-mode', mode);
        } else {
            var directTheme = getCookie('rfid_theme') || getStorage('rfid_theme');
            activeTheme = directTheme;

            if (!activeTheme) {
                var isDark = Boolean(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
                activeTheme = isDark ? (getCookie('rfid_theme_dark') || getStorage('rfid_theme_dark') || 'dark')
                                     : (getCookie('rfid_theme_light') || getStorage('rfid_theme_light') || 'corporate');
            }
        }

        var isDarkTheme = DARK_THEMES.indexOf(activeTheme) !== -1;
        if (isDarkTheme) {
            doc.classList.add('dark');
            doc.classList.remove('light');
        } else {
            doc.classList.add('light');
            doc.classList.remove('dark');
        }

        if (activeTheme) {
            doc.setAttribute('data-theme', activeTheme);
        }
    }

    syncTheme();
})();
