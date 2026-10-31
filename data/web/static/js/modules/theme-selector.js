/**
 * DaisyUI Theme & Mode Selector Module for RFID Desk
 * Supports 26 official DaisyUI themes (14 Light / 12 Dark),
 * Auto (system) / Light / Dark modes, interactive live component preview,
 * and Cookie / localStorage persistence.
 */

export const THEMES = [
    // --- Light Themes (20) ---
    { id: 'corporate', name: 'Corporate', type: 'light', desc: 'Professionell & Modern' },
    { id: 'emerald',   name: 'Emerald',   type: 'light', desc: 'Smaragdgrün & Frisch' },
    { id: 'nord',      name: 'Nord',      type: 'light', desc: 'Nordisch Klares Blau' },
    { id: 'cupcake',   name: 'Cupcake',   type: 'light', desc: 'Sanfte Pastellfarben' },
    { id: 'retro',     name: 'Retro',     type: 'light', desc: 'Vintage Beige / Retro' },
    { id: 'garden',    name: 'Garden',    type: 'light', desc: 'Natürliches Olivgrün' },
    { id: 'aqua',      name: 'Aqua',      type: 'light', desc: 'Ozean & Meeresblau' },
    { id: 'lofi',      name: 'Lofi',      type: 'light', desc: 'Minimal Monochrom' },
    { id: 'pastel',    name: 'Pastel',    type: 'light', desc: 'Bunte Pastelltöne' },
    { id: 'fantasy',   name: 'Fantasy',   type: 'light', desc: 'Verspielt & Pink' },
    { id: 'winter',    name: 'Winter',    type: 'light', desc: 'Klares Eisblau' },
    { id: 'autumn',    name: 'Autumn',    type: 'light', desc: 'Warme Herbsttöne' },
    { id: 'acid',      name: 'Acid',      type: 'light', desc: 'Neon & Säuregrün' },
    { id: 'lemonade',  name: 'Lemonade',  type: 'light', desc: 'Zitronengelb & Frisch' },
    { id: 'cyberpunk', name: 'Cyberpunk', type: 'light', desc: 'Gelber Cyber-Look' },
    { id: 'valentine', name: 'Valentine', type: 'light', desc: 'Romantisch Rosa' },
    { id: 'wireframe', name: 'Wireframe', type: 'light', desc: 'Drahtmodell Skizze' },
    { id: 'cmyk',      name: 'CMYK',      type: 'light', desc: 'Cyan Magenta Yellow' },
    { id: 'silk',      name: 'Silk',      type: 'light', desc: 'Sanftes Seidenweiss' },
    { id: 'light',     name: 'Light',     type: 'light', desc: 'Standard Hell' },

    // --- Dark Themes (13) ---
    { id: 'dark',      name: 'Dark',      type: 'dark',  desc: 'Standard Dunkel' },
    { id: 'synthwave', name: 'Synthwave', type: 'dark',  desc: 'Neon 80s Cyberpunk' },
    { id: 'halloween', name: 'Halloween', type: 'dark',  desc: 'Kürbisorange & Nacht' },
    { id: 'forest',    name: 'Forest',    type: 'dark',  desc: 'Tiefer Waldgrün' },
    { id: 'black',     name: 'Black',     type: 'dark',  desc: 'OLED Tiefschwarz' },
    { id: 'luxury',    name: 'Luxury',    type: 'dark',  desc: 'Schwarz & Gold' },
    { id: 'dracula',   name: 'Dracula',   type: 'dark',  desc: 'Vampir-Violett' },
    { id: 'business',  name: 'Business',  type: 'dark',  desc: 'Dunkelgrau & Business' },
    { id: 'night',     name: 'Night',     type: 'dark',  desc: 'Sternenklare Nacht' },
    { id: 'coffee',    name: 'Coffee',    type: 'dark',  desc: 'Warmes Kaffeebraun' },
    { id: 'dim',       name: 'Dim',       type: 'dark',  desc: 'Sanft Gedimmt' },
    { id: 'sunset',    name: 'Sunset',    type: 'dark',  desc: 'Sonnenuntergang Rot' },
    { id: 'abyss',     name: 'Abyss',     type: 'dark',  desc: 'Tiefsee Ozeanblau' }
];

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

export function isValidTheme(themeId, expectedType = null) {
    if (!themeId) return false;
    const theme = THEMES.find(t => t.id === themeId);
    return Boolean(theme && (!expectedType || theme.type === expectedType));
}

export function getStoredMode() {
    return getCookie('rfid_theme_mode') || getStorage('rfid_theme_mode') || 'auto';
}

export function getStoredLight() {
    const theme = getCookie('rfid_theme_light') || getStorage('rfid_theme_light');
    if (isValidTheme(theme, 'light')) return theme;
    return 'corporate';
}

export function getStoredDark() {
    const theme = getCookie('rfid_theme_dark') || getStorage('rfid_theme_dark');
    if (isValidTheme(theme, 'dark')) return theme;
    return 'dark';
}

export function getStoredTheme() {
    return getCookie('rfid_theme') || getStorage('rfid_theme') || null;
}

export function getEffectiveType(mode) {
    if (mode === 'darkmode' || mode === 'dark') return 'dark';
    if (mode === 'lightmode' || mode === 'light') return 'light';
    const isSystemDark = Boolean(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
    return isSystemDark ? 'dark' : 'light';
}

/**
 * Applies the effective theme and mode to document.documentElement
 */
export function applyTheme(mode, specificTheme = null) {
    const effectiveMode = mode || getStoredMode();
    const effectiveType = getEffectiveType(effectiveMode);
    const isDark = (effectiveType === 'dark');

    // 1. Update .dark / .light classes on <html>
    if (isDark) {
        document.documentElement.classList.add('dark');
        document.documentElement.classList.remove('light');
    } else {
        document.documentElement.classList.add('light');
        document.documentElement.classList.remove('dark');
    }

    // 2. Resolve target theme strictly matching effectiveType (light or dark)
    let targetTheme = null;
    if (specificTheme && isValidTheme(specificTheme, effectiveType)) {
        targetTheme = specificTheme;
    } else if (effectiveMode === 'auto') {
        targetTheme = isDark ? getStoredDark() : getStoredLight();
    } else if (isDark) {
        targetTheme = getStoredDark();
    } else {
        targetTheme = getStoredLight();
    }

    if (!targetTheme || !isValidTheme(targetTheme, effectiveType)) {
        targetTheme = isDark ? 'dark' : 'corporate';
    }

    document.documentElement.setAttribute('data-theme', targetTheme);

    // 3. Update header indicators if present
    updateHeaderIndicators(effectiveMode, targetTheme);

    // 4. Update preview components if modal is present
    updateLivePreview(targetTheme);

    return targetTheme;
}

function updateHeaderIndicators(mode, themeId) {
    const modeBadge = document.getElementById('theme-mode-indicator');
    const nameBadge = document.getElementById('theme-name-indicator');

    if (modeBadge) {
        if (mode === 'darkmode' || mode === 'dark') {
            modeBadge.textContent = 'Dunkel';
        } else if (mode === 'lightmode' || mode === 'light') {
            modeBadge.textContent = 'Hell';
        } else {
            modeBadge.textContent = 'Auto';
        }
    }

    if (nameBadge) {
        const themeObj = THEMES.find(t => t.id === themeId);
        nameBadge.textContent = themeObj ? themeObj.name : themeId;
    }
}

function updateLivePreview(themeId) {
    const titleEl = document.getElementById('live-preview-theme-title');
    if (titleEl) {
        const themeObj = THEMES.find(t => t.id === themeId);
        titleEl.textContent = themeObj ? themeObj.name : themeId;
    }
}

/**
 * Initializes Theme Dialog Modal and Theme Grid Event Listeners
 */
export function initThemeSelector() {
    const dialogBtn = document.getElementById('theme-dialog-btn');
    const dialog = document.getElementById('theme-modal-dialog');
    const closeBtn = document.getElementById('theme-modal-close');
    const applyCloseBtn = document.getElementById('theme-apply-close-btn');
    const resetBtn = document.getElementById('theme-reset-btn');
    const modeButtons = document.querySelectorAll('.theme-mode-btn');
    const gridContainer = document.getElementById('theme-grid');

    let currentMode = getStoredMode();

    // Synchronize initial theme & indicators
    applyTheme(currentMode);

    // Render Theme Cards in Modal Grid (strictly matching currentMode / light or dark)
    function renderGrid() {
        if (!gridContainer) return;

        const effectiveType = getEffectiveType(currentMode);
        const currentActiveTheme = document.documentElement.getAttribute('data-theme');
        gridContainer.innerHTML = '';

        // Update toolbar indicators
        const summaryTextEl = document.getElementById('theme-mode-summary-text');
        const countVisibleEl = document.getElementById('theme-count-visible');
        const isSystemDark = Boolean(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);

        if (summaryTextEl) {
            if (currentMode === 'auto') {
                summaryTextEl.textContent = isSystemDark ? 'Auto (System: Dunkel)' : 'Auto (System: Hell)';
            } else if (currentMode === 'darkmode' || currentMode === 'dark') {
                summaryTextEl.textContent = 'Dunkle Themes';
            } else {
                summaryTextEl.textContent = 'Helle Themes';
            }
        }

        // Only themes matching the active mode (light or dark)
        const modeMatchedThemes = THEMES.filter(t => t.type === effectiveType);

        if (countVisibleEl) {
            countVisibleEl.textContent = modeMatchedThemes.length;
        }

        if (modeMatchedThemes.length === 0) {
            gridContainer.innerHTML = `<div class="col-span-full py-6 text-center text-xs app-meta-text">Keine passenden Themes gefunden.</div>`;
            return;
        }

        modeMatchedThemes.forEach(t => {
            const isSelected = (t.id === currentActiveTheme);
            const card = document.createElement('div');
            card.setAttribute('data-theme', t.id);
            card.setAttribute('data-theme-id', t.id);
            card.setAttribute('data-theme-type', t.type);
            card.setAttribute('role', 'button');
            card.setAttribute('tabindex', '0');
            card.setAttribute('title', `${t.name} (${t.type === 'dark' ? 'Dunkel' : 'Hell'}): ${t.desc}`);
            card.className = `theme-card card bg-base-100 text-base-content border ${isSelected ? 'border-primary ring-2 ring-primary/40' : 'border-base-200 hover:border-base-content/20'} shadow-sm hover:shadow-md transition-all cursor-pointer p-3 select-none flex flex-col justify-between`;

            card.innerHTML = `
                <div class="flex items-center justify-between mb-2">
                    <div class="flex items-center gap-1.5">
                        <span class="text-xs font-bold">${t.name}</span>
                        <span class="badge badge-xs ${t.type === 'dark' ? 'badge-neutral' : 'badge-ghost'} text-[9px] uppercase font-mono">${t.type === 'dark' ? 'Dunkel' : 'Hell'}</span>
                    </div>
                    <span class="theme-check-icon ${isSelected ? '' : 'hidden'} text-primary font-bold text-xs shrink-0" aria-label="Ausgewählt">✓</span>
                </div>
                <div class="flex items-center justify-between mt-1 gap-2">
                    <span class="text-[10px] text-base-content/70 truncate max-w-[130px]">${t.desc}</span>
                    <div class="flex items-center gap-1 shrink-0" aria-hidden="true">
                        <span class="w-3.5 h-3.5 rounded-full bg-primary inline-block"></span>
                        <span class="w-3.5 h-3.5 rounded-full bg-secondary inline-block"></span>
                        <span class="w-3.5 h-3.5 rounded-full bg-accent inline-block"></span>
                        <span class="w-3.5 h-3.5 rounded-full bg-neutral inline-block"></span>
                    </div>
                </div>
            `;

            // Click Handler for Instant Live Test & Selection
            const selectThisTheme = () => {
                const selectedId = t.id;
                if (t.type === 'dark') {
                    setCookie('rfid_theme_dark', selectedId);
                    setStorage('rfid_theme_dark', selectedId);
                } else {
                    setCookie('rfid_theme_light', selectedId);
                    setStorage('rfid_theme_light', selectedId);
                }

                setCookie('rfid_theme', selectedId);
                setStorage('rfid_theme', selectedId);

                applyTheme(currentMode, selectedId);
                renderGrid();
            };

            card.addEventListener('click', selectThisTheme);
            card.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    selectThisTheme();
                }
            });

            gridContainer.appendChild(card);
        });
    }

    function updateModeButtonsUI(mode) {
        modeButtons.forEach(btn => {
            const btnMode = btn.getAttribute('data-mode');
            const isActive = (btnMode === mode);
            if (isActive) {
                btn.classList.add('btn-primary');
                btn.classList.remove('btn-ghost');
                btn.setAttribute('aria-checked', 'true');
            } else {
                btn.classList.remove('btn-primary');
                btn.classList.add('btn-ghost');
                btn.setAttribute('aria-checked', 'false');
            }
        });
    }

    // Modal Trigger
    if (dialogBtn && dialog) {
        dialogBtn.addEventListener('click', () => {
            currentMode = getStoredMode();
            updateModeButtonsUI(currentMode);
            applyTheme(currentMode);
            renderGrid();
            if (typeof dialog.showModal === 'function') {
                dialog.showModal();
            } else {
                dialog.setAttribute('open', '');
            }
        });
    }

    // Close Modal
    const closeModal = () => {
        if (dialog) {
            if (typeof dialog.close === 'function') {
                dialog.close();
            } else {
                dialog.removeAttribute('open');
            }
        }
    };

    if (closeBtn) closeBtn.addEventListener('click', closeModal);
    if (applyCloseBtn) applyCloseBtn.addEventListener('click', closeModal);

    // Mode Buttons Click
    modeButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            const selectedMode = btn.getAttribute('data-mode');
            currentMode = selectedMode;
            updateModeButtonsUI(currentMode);

            setCookie('rfid_theme_mode', selectedMode);
            setStorage('rfid_theme_mode', selectedMode);

            // Re-apply theme based on new mode
            applyTheme(selectedMode);
            renderGrid();
        });
    });

    // Search Input
    if (searchInput) {
        searchInput.addEventListener('input', (e) => {
            searchQuery = (e.target.value || '').trim().toLowerCase();
            renderGrid();
        });
    }

    // Reset Defaults Button
    if (resetBtn) {
        resetBtn.addEventListener('click', () => {
            currentMode = 'auto';
            setCookie('rfid_theme_mode', 'auto');
            setStorage('rfid_theme_mode', 'auto');

            setCookie('rfid_theme_light', 'corporate');
            setStorage('rfid_theme_light', 'corporate');

            setCookie('rfid_theme_dark', 'dark');
            setStorage('rfid_theme_dark', 'dark');

            setCookie('rfid_theme', 'corporate');
            setStorage('rfid_theme', 'corporate');

            updateModeButtonsUI('auto');
            applyTheme('auto');
            renderGrid();
        });
    }

    // Media query listener for Auto mode
    if (window.matchMedia) {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
            if (getStoredMode() === 'auto') {
                applyTheme('auto');
                renderGrid();
            }
        });
    }

    // Initial render if grid container is present
    renderGrid();
}

// Backward compatibility alias for skin-selector
export const initSkinSelector = initThemeSelector;
