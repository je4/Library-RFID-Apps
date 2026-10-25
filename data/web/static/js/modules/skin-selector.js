/**
 * Skin Selector Module for RFID Desk
 * Handles theme switching, data-skin DOM updates, and cookie/localStorage persistence.
 */

export function initSkinSelector() {
    const skinSelect = document.getElementById('skin-select');
    if (!skinSelect) return;

    // Synchronize select value with currently applied skin or default
    const currentSkin = document.documentElement.getAttribute('data-skin') || 'default';
    skinSelect.value = currentSkin;

    skinSelect.addEventListener('change', (e) => {
        const selectedSkin = e.target.value;

        // 1. Immediately apply to DOM
        if (selectedSkin && selectedSkin !== 'default') {
            document.documentElement.setAttribute('data-skin', selectedSkin);
        } else {
            document.documentElement.removeAttribute('data-skin');
        }

        // 2. Persist in Cookie (1 year duration, path=/, SameSite=Lax)
        document.cookie = `rfid_skin=${encodeURIComponent(selectedSkin)}; path=/; max-age=31536000; SameSite=Lax`;

        // 3. Fallback to localStorage
        try {
            localStorage.setItem('rfid_skin', selectedSkin);
        } catch (_) {}
    });
}
