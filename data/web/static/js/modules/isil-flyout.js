/**
 * ISIL Flyout Portal Module for RFID Desk
 * Positions floating tooltips for ISIL library metadata without overflow clipping.
 */

let globalIsilFlyout = null;
let activeFlyoutWrapper = null;
let flyoutHideTimer = null;

export function getOrCreateGlobalFlyout() {
    if (!globalIsilFlyout) {
        globalIsilFlyout = document.getElementById('app-global-isil-flyout');
        if (!globalIsilFlyout) {
            globalIsilFlyout = document.createElement('div');
            globalIsilFlyout.id = 'app-global-isil-flyout';
            globalIsilFlyout.className = 'app-isil-flyout';
            document.body.appendChild(globalIsilFlyout);
        }
    }
    return globalIsilFlyout;
}

export function showIsilFlyout(wrapper) {
    if (!wrapper) return;
    const template = wrapper.querySelector('.app-isil-flyout-template, template');
    if (!template) return;

    if (flyoutHideTimer) {
        clearTimeout(flyoutHideTimer);
        flyoutHideTimer = null;
    }

    const flyout = getOrCreateGlobalFlyout();
    activeFlyoutWrapper = wrapper;
    flyout.innerHTML = template.innerHTML;

    const btn = wrapper.querySelector('.app-isil-info-btn') || wrapper;
    const rect = btn.getBoundingClientRect();

    // If trigger button is out of viewport, hide
    if (rect.bottom < 0 || rect.top > window.innerHeight) {
        hideIsilFlyout(true);
        return;
    }

    // Measure natural dimensions in hidden state
    flyout.style.display = 'block';
    flyout.style.visibility = 'hidden';
    flyout.style.maxHeight = 'none';

    const flyoutWidth = Math.min(420, window.innerWidth - 32);
    flyout.style.width = `${flyoutWidth}px`;

    const naturalHeight = flyout.offsetHeight || 300;
    const spaceBelow = window.innerHeight - rect.bottom - 12;
    const spaceAbove = rect.top - 12;

    let top;
    let maxHeight;

    if (spaceBelow >= Math.min(naturalHeight, 220) || spaceBelow >= spaceAbove) {
        // Place below button
        top = rect.bottom + 6;
        maxHeight = Math.max(160, spaceBelow - 6);
    } else {
        // Place above button
        maxHeight = Math.max(160, spaceAbove - 6);
        const actualFlyoutHeight = Math.min(naturalHeight, maxHeight);
        top = Math.max(8, rect.top - actualFlyoutHeight - 6);
    }

    // Horizontal alignment: align with button left edge, keep within screen bounds
    let left = rect.left - 4;
    if (left + flyoutWidth > window.innerWidth - 16) {
        left = window.innerWidth - flyoutWidth - 16;
    }
    if (left < 16) {
        left = 16;
    }

    flyout.style.top = `${Math.round(top)}px`;
    flyout.style.left = `${Math.round(left)}px`;
    flyout.style.maxHeight = `${Math.round(maxHeight)}px`;
    flyout.style.visibility = 'visible';
    flyout.classList.add('is-visible');
}

export function scheduleHideIsilFlyout(delay = 180) {
    if (flyoutHideTimer) clearTimeout(flyoutHideTimer);
    flyoutHideTimer = setTimeout(() => {
        hideIsilFlyout();
    }, delay);
}

export function hideIsilFlyout(immediate = false) {
    if (flyoutHideTimer) {
        clearTimeout(flyoutHideTimer);
        flyoutHideTimer = null;
    }
    activeFlyoutWrapper = null;
    const flyout = getOrCreateGlobalFlyout();
    if (flyout) {
        flyout.classList.remove('is-visible');
        if (immediate) {
            flyout.style.display = 'none';
        } else {
            setTimeout(() => {
                if (!flyout.classList.contains('is-visible')) {
                    flyout.style.display = 'none';
                }
            }, 160);
        }
    }
}

export function initIsilFlyout() {
    getOrCreateGlobalFlyout();

    // Event listeners for ISIL mouseover / focus / hover bridge
    document.addEventListener('mouseover', (e) => {
        const wrapper = e.target.closest('.app-isil-info-wrapper');
        if (wrapper) {
            showIsilFlyout(wrapper);
            return;
        }
        if (e.target.closest('#app-global-isil-flyout')) {
            if (flyoutHideTimer) {
                clearTimeout(flyoutHideTimer);
                flyoutHideTimer = null;
            }
            return;
        }
        if (activeFlyoutWrapper && !e.target.closest('.app-isil-info-wrapper, #app-global-isil-flyout')) {
            scheduleHideIsilFlyout(120);
        }
    });

    document.addEventListener('mouseout', (e) => {
        const wrapper = e.target.closest('.app-isil-info-wrapper');
        if (wrapper) {
            scheduleHideIsilFlyout(180);
            return;
        }
        if (e.target.closest('#app-global-isil-flyout')) {
            scheduleHideIsilFlyout(180);
        }
    });

    document.addEventListener('focusin', (e) => {
        const wrapper = e.target.closest('.app-isil-info-wrapper');
        if (wrapper) {
            showIsilFlyout(wrapper);
        }
    });

    document.addEventListener('focusout', (e) => {
        const wrapper = e.target.closest('.app-isil-info-wrapper');
        if (wrapper) {
            scheduleHideIsilFlyout(150);
        }
    });

    // Close on Escape or click outside
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') hideIsilFlyout(true);
    });

    document.addEventListener('click', (e) => {
        if (!e.target.closest('.app-isil-info-wrapper, #app-global-isil-flyout')) {
            hideIsilFlyout(true);
        }
    });

    // Reposition on scroll / window resize
    window.addEventListener('scroll', () => {
        if (activeFlyoutWrapper && globalIsilFlyout && globalIsilFlyout.classList.contains('is-visible')) {
            showIsilFlyout(activeFlyoutWrapper);
        }
    }, { passive: true, capture: true });

    window.addEventListener('resize', () => {
        if (activeFlyoutWrapper && globalIsilFlyout && globalIsilFlyout.classList.contains('is-visible')) {
            showIsilFlyout(activeFlyoutWrapper);
        }
    }, { passive: true });
}
