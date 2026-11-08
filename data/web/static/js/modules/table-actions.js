/**
 * Table Actions Module for RFID Desk
 * Handles filtering, clipboard copy operations, relative timestamp updates,
 * AFI row color highlights, multi-part set completeness checks, and dynamic bulk action counters.
 */

export function applyFilter() {
    const filterInput = document.getElementById('tag-filter-input');
    const query = (filterInput ? filterInput.value : '').toLowerCase().trim();
    const rows = document.querySelectorAll('#tags-table-body tr.tag-row');
    rows.forEach((row) => {
        const text = row.textContent.toLowerCase();
        if (!query || text.includes(query)) {
            row.style.display = '';
        } else {
            row.style.display = 'none';
        }
    });
}

export function updateTimeAgoLabels() {
    const timeElements = document.querySelectorAll('.time-ago');
    const now = Math.floor(Date.now() / 1000);
    timeElements.forEach((el) => {
        const ts = parseInt(el.getAttribute('data-timestamp'), 10);
        if (ts > 0) {
            const diff = now - Math.floor(ts / 1000);
            if (diff < 5) {
                el.textContent = 'gerade eben';
            } else if (diff < 60) {
                el.textContent = 'vor ' + diff + 's';
            } else if (diff < 3600) {
                el.textContent = 'vor ' + Math.floor(diff / 60) + 'm';
            } else {
                el.textContent = 'vor ' + Math.floor(diff / 3600) + 'h';
            }
        }
    });
}

/**
 * Update loan and return table enhancements:
 * - Colorizes rows based on security status (C2 / 07 / error)
 * - Verifies multi-part completeness (checks if all parts for an ItemID are present on pad)
 * - Dynamically updates labels and disabled states for bulk action buttons (Entsichern / Sichern)
 */
export function updateLoanTableEnhancements() {
    const loanTableBody = document.getElementById('loan-table-body');
    if (!loanTableBody) return;

    const rows = Array.from(loanTableBody.querySelectorAll('tr.tag-row'));
    const itemGroups = new Map();

    rows.forEach((row) => {
        const itemId = (row.getAttribute('data-itemid') || '').trim();
        const partNo = parseInt(row.getAttribute('data-partno') || '1', 10);
        const parts = parseInt(row.getAttribute('data-parts') || '1', 10);
        const afi = (row.getAttribute('data-afi') || '').toUpperCase().trim();
        const isSecured = row.getAttribute('data-secured') === 'true' || afi === '07';
        const isUnsecured = row.getAttribute('data-unsecured') === 'true' || afi === 'C2';
        const isCrcOk = row.getAttribute('data-crcok') !== 'false';
        const isWriteError = row.querySelector('.badge-error') !== null;
        const hasError = isWriteError || !isCrcOk;

        // Apply row color styles
        row.classList.remove('loan-row-unsecured', 'loan-row-secured', 'loan-row-error');
        if (hasError) {
            row.classList.add('loan-row-error');
        } else if (isUnsecured) {
            row.classList.add('loan-row-unsecured');
        } else if (isSecured) {
            row.classList.add('loan-row-secured');
        }

        if (itemId && itemId !== 'Keine ItemID') {
            if (!itemGroups.has(itemId)) {
                itemGroups.set(itemId, { totalParts: parts, rows: [], partNumbers: new Set() });
            }
            const group = itemGroups.get(itemId);
            group.rows.push(row);
            group.partNumbers.add(partNo);
            if (parts > group.totalParts) group.totalParts = parts;
        }
    });

    // Update multi-part completeness indicators
    itemGroups.forEach((group) => {
        const isComplete = group.partNumbers.size >= group.totalParts;
        group.rows.forEach((row) => {
            const statusEl = row.querySelector('.loan-part-status');
            const indicatorEl = row.querySelector('.loan-part-indicator');
            const isCrcOk = row.getAttribute('data-crcok') !== 'false';
            const isWriteError = row.querySelector('.badge-error') !== null;
            const hasError = isWriteError || !isCrcOk;

            if (statusEl && indicatorEl && group.totalParts > 1) {
                indicatorEl.classList.remove('part-complete', 'part-incomplete', 'part-error');

                if (hasError) {
                    indicatorEl.classList.add('part-error');
                    statusEl.innerHTML = `<span class="app-icon icon-x-mark text-error w-3.5 h-3.5 inline-block shrink-0" aria-hidden="true"></span>`;
                    indicatorEl.setAttribute('title', `Fehlerhafter Transponder / Lesefehler bei diesem Teil`);
                } else if (isComplete) {
                    indicatorEl.classList.add('part-complete');
                    statusEl.innerHTML = `<span class="app-icon icon-check text-success w-3.5 h-3.5 inline-block shrink-0" aria-hidden="true"></span>`;
                    indicatorEl.setAttribute('title', `Vollständiges Set: Alle ${group.totalParts} Teile liegen auf dem Lesegerät`);
                } else {
                    indicatorEl.classList.add('part-incomplete');
                    statusEl.innerHTML = `<span class="app-icon icon-alert-circle text-warning w-3.5 h-3.5 inline-block shrink-0" aria-hidden="true"></span><span class="text-[11px] text-base-content font-bold">(${group.partNumbers.size}/${group.totalParts})</span>`;
                    indicatorEl.setAttribute('title', `Achtung: Unvollständiges Set (${group.partNumbers.size} von ${group.totalParts} Teilen liegen auf dem Lesegerät)`);
                }
            } else if (statusEl) {
                statusEl.innerHTML = '';
            }
        });
    });

    // Update bulk action buttons (Entsichern / Sichern)
    const unsecureBtn = document.getElementById('btn-unsecure');
    const unsecureLabel = document.getElementById('btn-unsecure-label');
    const secureBtn = document.getElementById('btn-secure');
    const secureLabel = document.getElementById('btn-secure-label');

    const totalCount = rows.length;
    let countNotUnsecured = 0;
    let countNotSecured = 0;

    rows.forEach((row) => {
        const afi = (row.getAttribute('data-afi') || '').toUpperCase().trim();
        const isSecured = row.getAttribute('data-secured') === 'true' || afi === '07';
        const isUnsecured = row.getAttribute('data-unsecured') === 'true' || afi === 'C2';
        if (!isUnsecured) countNotUnsecured++;
        if (!isSecured) countNotSecured++;
    });

    if (unsecureBtn) {
        if (totalCount === 0) {
            unsecureBtn.disabled = true;
            if (unsecureLabel) unsecureLabel.textContent = 'Entsichern';
        } else if (countNotUnsecured > 0) {
            unsecureBtn.disabled = false;
            if (unsecureLabel) unsecureLabel.textContent = totalCount === 1 ? '1 Medium entsichern (C2)' : `Alle ${totalCount} Medien entsichern (C2)`;
        } else {
            unsecureBtn.disabled = true;
            if (unsecureLabel) unsecureLabel.textContent = 'Bereits alle entsichert (C2)';
        }
    }

    if (secureBtn) {
        if (totalCount === 0) {
            secureBtn.disabled = true;
            if (secureLabel) secureLabel.textContent = 'Sichern';
        } else if (countNotSecured > 0) {
            secureBtn.disabled = false;
            if (secureLabel) secureLabel.textContent = totalCount === 1 ? '1 Medium sichern (07)' : `Alle ${totalCount} Medien sichern (07)`;
        } else {
            secureBtn.disabled = true;
            if (secureLabel) secureLabel.textContent = 'Bereits alle gesichert (07)';
        }
    }
}

export function initTableActions() {
    // Filter input
    const filterInput = document.getElementById('tag-filter-input');
    if (filterInput) {
        filterInput.addEventListener('input', applyFilter);
    }

    // Copy all Item-IDs button for loan & return page
    const copyAllBtn = document.getElementById('btn-copy-all-itemids');
    if (copyAllBtn) {
        copyAllBtn.addEventListener('click', () => {
            const itemElements = document.querySelectorAll('#loan-table-body .loan-item-id, #loan-table-body tr.tag-row[data-itemid]');
            const itemIds = [];
            itemElements.forEach((el) => {
                let id = el.getAttribute('data-itemid') || el.textContent;
                id = id ? id.trim() : '';
                if (id && id !== 'Keine ItemID' && !itemIds.includes(id)) {
                    itemIds.push(id);
                }
            });

            const copyLabel = document.getElementById('copy-all-label');
            const originalLabel = copyLabel ? copyLabel.textContent : 'Item-IDs kopieren';

            if (itemIds.length === 0) {
                if (copyLabel) copyLabel.textContent = 'Keine IDs vorhanden';
                setTimeout(() => {
                    if (copyLabel) copyLabel.textContent = originalLabel;
                }, 2000);
                return;
            }

            const clipboardText = itemIds.join('\n');
            if (navigator.clipboard) {
                navigator.clipboard.writeText(clipboardText).then(() => {
                    if (copyLabel) copyLabel.textContent = `Kopiert (${itemIds.length})`;
                    copyAllBtn.classList.add('copied', 'btn-success');
                    setTimeout(() => {
                        if (copyLabel) copyLabel.textContent = originalLabel;
                        copyAllBtn.classList.remove('copied', 'btn-success');
                    }, 2000);
                }).catch((err) => {
                    console.error('Clipboard copy failed:', err);
                });
            }
        });
    }

    // Copy UID / ItemID to clipboard helper (event delegation)
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('.copy-uid-btn, .copy-itemid-btn');
        if (btn) {
            const textToCopy = btn.getAttribute('data-uid') || btn.getAttribute('data-itemid') || btn.getAttribute('data-copy');
            if (textToCopy && navigator.clipboard) {
                navigator.clipboard.writeText(textToCopy).then(() => {
                    const originalHtml = btn.innerHTML;
                    btn.classList.add('btn-active');
                    btn.innerHTML = '<span class="text-success text-xs font-bold">Kopiert!</span>';
                    setTimeout(() => {
                        btn.innerHTML = originalHtml;
                        btn.classList.remove('btn-active');
                    }, 1500);
                });
            }
        }
    });

    // Run initial enhancements
    updateLoanTableEnhancements();

    // Auto-refresh relative time labels every 5 seconds
    setInterval(updateTimeAgoLabels, 5000);
}
