/**
 * Early Theme & Skin Initializer for RFID Desk (FOUC Prevention).
 * Executes synchronously in the <head> before rendering to prevent layout/theme flashes.
 */
(function() {
    var themeMap = {
        'default':   { light: 'corporate', dark: 'dark' },
        'emerald':   { light: 'emerald',   dark: 'emerald' },
        'nord':      { light: 'nord',      dark: 'nord' },
        'bubblegum': { light: 'cupcake',   dark: 'synthwave' }
    };

    function getSavedSkin() {
        var match = document.cookie.match(/(?:^|;\s*)rfid_skin=([^;]+)/);
        if (match) return decodeURIComponent(match[1]);
        try { return localStorage.getItem('rfid_skin'); } catch(e) { return null; }
    }

    function getSavedMode() {
        var match = document.cookie.match(/(?:^|;\s*)rfid_theme_mode=([^;]+)/);
        if (match) return decodeURIComponent(match[1]);
        try { return localStorage.getItem('rfid_theme_mode'); } catch(e) { return null; }
    }

    var skin = getSavedSkin() || 'default';
    if (skin && skin !== 'default') {
        document.documentElement.setAttribute('data-skin', skin);
    } else {
        document.documentElement.removeAttribute('data-skin');
    }

    function syncTheme() {
        var mode = getSavedMode() || 'auto';
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

        var currentSkin = document.documentElement.getAttribute('data-skin') || 'default';
        var mapping = themeMap[currentSkin] || themeMap['default'];
        document.documentElement.setAttribute('data-theme', isDark ? mapping.dark : mapping.light);
    }

    syncTheme();

    if (window.matchMedia) {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function() {
            var mode = getSavedMode() || 'auto';
            if (mode === 'auto') syncTheme();
        });
    }
})();
