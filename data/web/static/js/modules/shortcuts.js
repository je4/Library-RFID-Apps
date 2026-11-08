/**
 * Keyboard Shortcuts Module for RFID Desk
 * Ergonomic hotkeys for high-efficiency library circulation and desk workflows.
 */

export function initShortcuts() {
    document.addEventListener('keydown', (e) => {
        // Do not trigger global Alt shortcuts if user is inside an input, unless specific keys
        const targetTag = e.target.tagName ? e.target.tagName.toLowerCase() : '';
        const isInput = targetTag === 'input' || targetTag === 'textarea' || targetTag === 'select';

        // F2: Focus main input field (Barcode or Filter)
        if (e.key === 'F2') {
            e.preventDefault();
            const barcodeInput = document.getElementById('encode-item-id');
            const filterInput = document.getElementById('tag-filter-input');
            if (barcodeInput) {
                barcodeInput.focus();
                if (typeof barcodeInput.select === 'function') barcodeInput.select();
            } else if (filterInput) {
                filterInput.focus();
                if (typeof filterInput.select === 'function') filterInput.select();
            }
            return;
        }

        // Alt + 1: Switch to Desk view
        if (e.altKey && (e.key === '1' || e.code === 'Digit1')) {
            e.preventDefault();
            const deskBtn = document.getElementById('nav-desk-btn');
            if (deskBtn && deskBtn.getAttribute('href') && deskBtn.getAttribute('href') !== '#') {
                window.location.href = deskBtn.getAttribute('href');
            }
            return;
        }

        // Alt + 2: Switch to Loan & Return view
        if (e.altKey && (e.key === '2' || e.code === 'Digit2')) {
            e.preventDefault();
            const loanBtn = document.getElementById('nav-loan-btn');
            if (loanBtn && loanBtn.getAttribute('href') && loanBtn.getAttribute('href') !== '#') {
                window.location.href = loanBtn.getAttribute('href');
            }
            return;
        }

        // Alt + 3: Switch to Encode view
        if (e.altKey && (e.key === '3' || e.code === 'Digit3')) {
            e.preventDefault();
            const encodeBtn = document.getElementById('nav-encode-btn');
            if (encodeBtn && encodeBtn.getAttribute('href') && encodeBtn.getAttribute('href') !== '#') {
                window.location.href = encodeBtn.getAttribute('href');
            }
            return;
        }

        // Alt + U: Unsecure (AFI C2)
        if (e.altKey && (e.key === 'u' || e.key === 'U' || e.code === 'KeyU')) {
            e.preventDefault();
            const unsecureBtn = document.getElementById('btn-unsecure');
            if (unsecureBtn && !unsecureBtn.disabled) {
                unsecureBtn.click();
            }
            return;
        }

        // Alt + S: Secure (AFI 07)
        if (e.altKey && (e.key === 's' || e.key === 'S' || e.code === 'KeyS')) {
            e.preventDefault();
            const secureBtn = document.getElementById('btn-secure');
            if (secureBtn && !secureBtn.disabled) {
                secureBtn.click();
            }
            return;
        }

        // Alt + C: Copy all Item IDs
        if (e.altKey && (e.key === 'c' || e.key === 'C' || e.code === 'KeyC')) {
            // Avoid conflict with Ctrl+C if only Alt is pressed
            if (!e.ctrlKey && !e.metaKey) {
                const copyBtn = document.getElementById('btn-copy-all-itemids');
                if (copyBtn) {
                    e.preventDefault();
                    copyBtn.click();
                }
            }
            return;
        }

        // Alt + T: Toggle Audio Feedback (Ton ein / aus)
        if (e.altKey && (e.key === 't' || e.key === 'T' || e.code === 'KeyT')) {
            e.preventDefault();
            const soundToggle = document.getElementById('sound-toggle');
            if (soundToggle) {
                soundToggle.click();
            }
            return;
        }

        // Escape: Close open modals / blur filter
        if (e.key === 'Escape') {
            const openModals = document.querySelectorAll('dialog.modal[open], dialog.modal-open');
            if (openModals.length > 0) {
                openModals.forEach((modal) => {
                    if (typeof modal.close === 'function') {
                        modal.close();
                    } else {
                        modal.removeAttribute('open');
                        modal.classList.remove('modal-open');
                    }
                });
            } else if (isInput) {
                e.target.blur();
            }
        }
    });
}
