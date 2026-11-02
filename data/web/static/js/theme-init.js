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
        var directTheme = getCookie('rfid_theme') || getStorage('rfid_theme');
        var activeTheme = directTheme;

        if (!activeTheme) {
            var isDark = Boolean(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
            activeTheme = isDark ? (getCookie('rfid_theme_dark') || getStorage('rfid_theme_dark') || 'dark')
                                 : (getCookie('rfid_theme_light') || getStorage('rfid_theme_light') || 'corporate');
        }

        var isDarkTheme = DARK_THEMES.indexOf(activeTheme) !== -1;
        if (isDarkTheme) {
            document.documentElement.classList.add('dark');
            document.documentElement.classList.remove('light');
        } else {
            document.documentElement.classList.add('light');
            document.documentElement.classList.remove('dark');
        }

        document.documentElement.setAttribute('data-theme', activeTheme);
    }

    syncTheme();
})();
