/**
 * DaisyUI Theme & Mode Selector Module for RFID Desk
 * Supports 26 official DaisyUI themes (14 Light / 12 Dark),
 * Auto (system) / Light / Dark modes, interactive live component preview,
 * and Cookie / localStorage persistence.
 */

export const THEMES = [
    // --- Light Themes (14) ---
    { id: 'corporate', name: 'Corporate', type: 'light', desc: 'Professionell & Modern', colors: ['#2563eb', '#64748b', '#059669', '#1e293b'] },
    { id: 'emerald',   name: 'Emerald',   type: 'light', desc: 'Smaragdgrün & Frisch', colors: ['#10b981', '#1e293b', '#f59e0b', '#f8fafc'] },
    { id: 'nord',      name: 'Nord',      type: 'light', desc: 'Nordisch Klares Blau', colors: ['#5e81ac', '#88c0d0', '#81a1c1', '#eceff4'] },
    { id: 'cupcake',   name: 'Cupcake',   type: 'light', desc: 'Sanfte Pastellfarben', colors: ['#65c3c8', '#ef9fbc', '#eeaf3a', '#faf7f5'] },
    { id: 'retro',     name: 'Retro',     type: 'light', desc: 'Vintage Beige / Retro', colors: ['#ef9995', '#a4cbb4', '#dc8850', '#ece3ca'] },
    { id: 'garden',    name: 'Garden',    type: 'light', desc: 'Natürliches Olivgrün', colors: ['#5c7f67', '#ecf4e7', '#e9e7e7', '#291e00'] },
    { id: 'aqua',      name: 'Aqua',      type: 'light', desc: 'Ozean & Meeresblau',   colors: ['#09ecf3', '#7209b7', '#f72585', '#345da7'] },
    { id: 'lofi',      name: 'Lofi',      type: 'light', desc: 'Minimal Monochrom',   colors: ['#0d0d0d', '#1a1919', '#262626', '#ffffff'] },
    { id: 'pastel',    name: 'Pastel',    type: 'light', desc: 'Bunte Pastelltöne',   colors: ['#d1c1d7', '#f6cbd1', '#b4e9d6', '#ffffff'] },
    { id: 'fantasy',   name: 'Fantasy',   type: 'light', desc: 'Verspielt & Pink',     colors: ['#6e0b75', '#007ebd', '#f8860d', '#ffffff'] },
    { id: 'winter',    name: 'Winter',    type: 'light', desc: 'Klares Eisblau',       colors: ['#047aff', '#463aa2', '#c148ac', '#ffffff'] },
    { id: 'autumn',    name: 'Autumn',    type: 'light', desc: 'Warme Herbsttöne',    colors: ['#8c0327', '#d85251', '#d59b6a', '#f1f1f1'] },
    { id: 'acid',      name: 'Acid',      type: 'light', desc: 'Neon & Säuregrün',     colors: ['#e2f22b', '#ff6b6b', '#111111', '#fafafa'] },
    { id: 'lemonade',  name: 'Lemonade',  type: 'light', desc: 'Zitronengelb & Frisch', colors: ['#519903', '#e9e92f', '#2b8f38', '#ffffff'] },
    { id: 'cyberpunk', name: 'Cyberpunk', type: 'light', desc: 'Gelber Cyber-Look',   colors: ['#ff7598', '#75d1f0', '#c07eec', '#ffee00'] },
    { id: 'valentine', name: 'Valentine', type: 'light', desc: 'Romantisch Rosa',     colors: ['#e96d7b', '#a991f7', '#88dbdd', '#f0d6e8'] },
    { id: 'wireframe', name: 'Wireframe', type: 'light', desc: 'Drahtmodell Skizze',  colors: ['#b8b8b8', '#888888', '#b8b8b8', '#ebebeb'] },
    { id: 'cmyk',      name: 'CMYK',      type: 'light', desc: 'Cyan Magenta Yellow',  colors: ['#45AEEE', '#E8488A', '#FFF232', '#1a1a1a'] },
    { id: 'silk',      name: 'Silk',      type: 'light', desc: 'Sanftes Seidenweiss',  colors: ['#1e293b', '#0284c7', '#0d9488', '#f8fafc'] },
    { id: 'light',     name: 'Light',     type: 'light', desc: 'Standard Hell',        colors: ['#570df8', '#f000b8', '#37cdbe', '#ffffff'] },

    // --- Dark Themes (12) ---
    { id: 'dark',      name: 'Dark',      type: 'dark',  desc: 'Standard Dunkel',      colors: ['#661ae6', '#d926aa', '#1fb2a5', '#191e24'] },
    { id: 'synthwave', name: 'Synthwave', type: 'dark',  desc: 'Neon 80s Cyberpunk',   colors: ['#e779c1', '#58c7f3', '#f3cc30', '#1a103c'] },
    { id: 'halloween', name: 'Halloween', type: 'dark',  desc: 'Kürbisorange & Nacht', colors: ['#f28c18', '#6d3a9c', '#51a800', '#212121'] },
    { id: 'forest',    name: 'Forest',    type: 'dark',  desc: 'Tiefer Waldgrün',      colors: ['#1eb854', '#1db88e', '#1db8ab', '#171212'] },
    { id: 'black',     name: 'Black',     type: 'dark',  desc: 'OLED Tiefschwarz',     colors: ['#343232', '#555555', '#777777', '#000000'] },
    { id: 'luxury',    name: 'Luxury',    type: 'dark',  desc: 'Schwarz & Gold',       colors: ['#ffffff', '#152747', '#513448', '#09090b'] },
    { id: 'dracula',   name: 'Dracula',   type: 'dark',  desc: 'Vampir-Violett',       colors: ['#ff79c6', '#bd93f9', '#ffb86c', '#282a36'] },
    { id: 'business',  name: 'Business',  type: 'dark',  desc: 'Dunkelgrau & Business', colors: ['#1C4E80', '#7D8FA9', '#A5D8FF', '#202020'] },
    { id: 'night',     name: 'Night',     type: 'dark',  desc: 'Sternenklare Nacht',   colors: ['#38bdf8', '#818cf8', '#f472b6', '#0f172a'] },
    { id: 'coffee',    name: 'Coffee',    type: 'dark',  desc: 'Warmes Kaffeebraun',   colors: ['#db924b', '#263e3f', '#9fe88d', '#20161f'] },
    { id: 'dim',       name: 'Dim',       type: 'dark',  desc: 'Sanft Gedimmt',        colors: ['#9fe88d', '#f3cc30', '#20161f', '#2a323c'] },
    { id: 'sunset',    name: 'Sunset',    type: 'dark',  desc: 'Sonnenuntergang Rot',  colors: ['#ff865b', '#fd6f9c', '#b387fa', '#1a1921'] },
    { id: 'abyss',     name: 'Abyss',     type: 'dark',  desc: 'Tiefsee Ozeanblau',    colors: ['#38bdf8', '#0284c7', '#0369a1', '#030712'] }
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

export function getStoredMode() {
    return getCookie('rfid_theme_mode') || getStorage('rfid_theme_mode') || 'auto';
}

export function getStoredLight() {
    return getCookie('rfid_theme_light') || getStorage('rfid_theme_light') || 'corporate';
}

export function getStoredDark() {
    return getCookie('rfid_theme_dark') || getStorage('rfid_theme_dark') || 'dark';
}

export function getStoredTheme() {
    return getCookie('rfid_theme') || getStorage('rfid_theme') || null;
}

/**
 * Applies the effective theme and mode to document.documentElement
 */
export function applyTheme(mode, specificTheme = null) {
    const effectiveMode = mode || getStoredMode();
    const isSystemDark = Boolean(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);

    let isDark = false;
    if (effectiveMode === 'darkmode' || effectiveMode === 'dark') {
        isDark = true;
    } else if (effectiveMode === 'lightmode' || effectiveMode === 'light') {
        isDark = false;
    } else {
        isDark = isSystemDark;
    }

    // 1. Update .dark / .light classes on <html>
    if (isDark) {
        document.documentElement.classList.add('dark');
        document.documentElement.classList.remove('light');
    } else {
        document.documentElement.classList.add('light');
        document.documentElement.classList.remove('dark');
    }

    // 2. Resolve target theme
    let targetTheme = specificTheme;
    if (!targetTheme) {
        if (effectiveMode === 'auto') {
            targetTheme = isDark ? getStoredDark() : getStoredLight();
        } else if (isDark) {
            targetTheme = getStoredDark();
        } else {
            targetTheme = getStoredLight();
        }
    }

    if (!targetTheme) {
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
    const filterButtons = document.querySelectorAll('.theme-filter-btn');
    const searchInput = document.getElementById('theme-search-input');
    const gridContainer = document.getElementById('theme-grid');

    let currentMode = getStoredMode();
    let currentTheme = document.documentElement.getAttribute('data-theme') || (currentMode === 'darkmode' ? getStoredDark() : getStoredLight());
    let activeFilter = 'all';
    let searchQuery = '';

    // Synchronize initial theme & indicators
    applyTheme(currentMode, currentTheme);

    // Render Theme Cards in Modal Grid
    function renderGrid() {
        if (!gridContainer) return;

        const currentActiveTheme = document.documentElement.getAttribute('data-theme');
        gridContainer.innerHTML = '';

        const filtered = THEMES.filter(t => {
            const matchesFilter = (activeFilter === 'all') || (activeFilter === t.type);
            const matchesSearch = !searchQuery || 
                t.name.toLowerCase().includes(searchQuery) || 
                t.id.toLowerCase().includes(searchQuery) || 
                t.desc.toLowerCase().includes(searchQuery);
            return matchesFilter && matchesSearch;
        });

        if (filtered.length === 0) {
            gridContainer.innerHTML = `<div class="col-span-full py-6 text-center text-xs app-meta-text">Keine Themes gefunden.</div>`;
            return;
        }

        filtered.forEach(t => {
            const isSelected = (t.id === currentActiveTheme);
            const card = document.createElement('div');
            card.className = `theme-card flex flex-col justify-between ${isSelected ? 'is-active' : ''}`;
            card.setAttribute('data-theme-id', t.id);
            card.setAttribute('data-theme-type', t.type);
            card.setAttribute('role', 'button');
            card.setAttribute('tabindex', '0');
            card.setAttribute('title', `${t.name} (${t.type === 'dark' ? 'Dunkel' : 'Hell'}): ${t.desc}`);

            card.innerHTML = `
                <div class="flex items-center justify-between mb-2">
                    <div class="flex items-center gap-1.5">
                        <span class="text-xs font-bold text-base-content">${t.name}</span>
                        <span class="badge badge-xs ${t.type === 'dark' ? 'badge-neutral' : 'badge-ghost'} text-[9px] uppercase font-mono">${t.type === 'dark' ? 'Dunkel' : 'Hell'}</span>
                    </div>
                    <span class="theme-check-icon ${isSelected ? '' : 'hidden'} text-primary font-bold text-xs shrink-0" aria-label="Ausgewählt">✓</span>
                </div>
                <div class="flex items-center justify-between mt-1">
                    <span class="text-[10px] app-meta-text truncate max-w-[130px]">${t.desc}</span>
                    <div class="theme-swatch-box" aria-hidden="true">
                        ${t.colors.map(c => `<span class="theme-swatch" style="background-color: ${c};"></span>`).join('')}
                    </div>
                </div>
            `;

            // Click Handler for Instant Live Test & Selection
            const selectThisTheme = () => {
                const selectedId = t.id;
                const isDarkTheme = (t.type === 'dark');

                // If user was in auto mode and clicks a dark theme or light theme, persist preferred light/dark
                if (isDarkTheme) {
                    setCookie('rfid_theme_dark', selectedId);
                    setStorage('rfid_theme_dark', selectedId);
                } else {
                    setCookie('rfid_theme_light', selectedId);
                    setStorage('rfid_theme_light', selectedId);
                }

                // If current mode was opposite, adapt mode or set direct theme
                if (currentMode === 'lightmode' && isDarkTheme) {
                    currentMode = 'darkmode';
                    setCookie('rfid_theme_mode', 'darkmode');
                    setStorage('rfid_theme_mode', 'darkmode');
                    updateModeButtonsUI('darkmode');
                } else if (currentMode === 'darkmode' && !isDarkTheme) {
                    currentMode = 'lightmode';
                    setCookie('rfid_theme_mode', 'lightmode');
                    setStorage('rfid_theme_mode', 'lightmode');
                    updateModeButtonsUI('lightmode');
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

    function updateFilterButtonsUI(filter) {
        filterButtons.forEach(btn => {
            const btnFilter = btn.getAttribute('data-filter');
            const isActive = (btnFilter === filter);
            if (isActive) {
                btn.classList.add('btn-primary');
                btn.classList.remove('btn-ghost');
            } else {
                btn.classList.remove('btn-primary');
                btn.classList.add('btn-ghost');
            }
        });
    }

    // Modal Trigger
    if (dialogBtn && dialog) {
        dialogBtn.addEventListener('click', () => {
            currentMode = getStoredMode();
            updateModeButtonsUI(currentMode);
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

    // Filter Buttons Click
    filterButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            activeFilter = btn.getAttribute('data-filter') || 'all';
            updateFilterButtonsUI(activeFilter);
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
            applyTheme('auto', 'corporate');
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
