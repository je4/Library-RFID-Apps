/**
 * Gate Alarm Notification & Management Module for RFID Desk
 * Listens for gate_alarm SSE events, provides acoustic feedback, displays floating alert toasts,
 * and renders detailed alarm lists with catalog metadata enrichment.
 */

import { playAlarmSound } from './audio.js';
import { t } from './i18n.js';

let activeAlarms = [];
let toastDismissedUntil = 0;

function apiURL(subpath) {
    let path = window.location.pathname.replace(/\/(?:loan|encode|inventory)\/?$/, '/');
    if (!path.endsWith('/')) path += '/';
    return new URL(path + subpath, window.location.origin).href;
}

export function getActiveAlarms() {
    return activeAlarms;
}

export function showAlarmToast(incident) {
    const toast = document.getElementById('gate-alarm-toast');
    if (!toast) return;

    const count = incident.totalCount || incident.items?.length || activeAlarms.length || 1;
    const gateName = incident.gate || incident.channel || incident.item?.gate || t('gateAlarm.gate');
    const timeStr = incident.timeString || incident.item?.timeString || new Date().toLocaleTimeString();

    const subtitle = document.getElementById('gate-alarm-toast-subtitle');
    if (subtitle) {
        if (count > 1) {
            subtitle.textContent = t('gateAlarm.subtitleMultiple', {
                count: count,
                gate: gateName,
                time: timeStr
            });
        } else {
            subtitle.textContent = t('gateAlarm.subtitle', {
                count: count,
                gate: gateName,
                time: timeStr
            });
        }
    }

    const badge = document.getElementById('gate-alarm-toast-badge');
    if (badge) {
        badge.textContent = count;
    }

    toast.classList.remove('hidden');

    const navIndicator = document.getElementById('nav-gate-alarm-indicator');
    if (navIndicator) {
        navIndicator.classList.remove('hidden');
    }
}

export function hideAlarmToast() {
    const toast = document.getElementById('gate-alarm-toast');
    if (toast) {
        toast.classList.add('hidden');
    }
    toastDismissedUntil = Date.now() + 60000;
}

export function openAlarmModal() {
    const modal = document.getElementById('gate-alarm-modal');
    if (modal && typeof modal.showModal === 'function') {
        renderAlarmTable();
        modal.showModal();
    }
}

export function renderAlarmTable() {
    const tbody = document.getElementById('gate-alarm-modal-tbody');
    const countBadge = document.getElementById('gate-alarm-modal-count');
    if (!tbody) return;

    if (countBadge) {
        countBadge.textContent = activeAlarms.length;
    }

    if (activeAlarms.length === 0) {
        tbody.innerHTML = `
            <tr id="gate-alarm-empty-row">
                <td colspan="5" class="text-center py-8 text-base-content/50 italic">
                    ${t('gateAlarm.empty')}
                </td>
            </tr>
        `;
        const navIndicator = document.getElementById('nav-gate-alarm-indicator');
        if (navIndicator) {
            navIndicator.classList.add('hidden');
        }
        return;
    }

    const rows = activeAlarms.map(item => {
        const barcode = item.barcode || item.uid || '-';
        const uid = item.uid || '-';
        const gate = item.gate || item.channel || '-';
        const timeStr = item.timeString || (item.time ? new Date(item.time).toLocaleTimeString() : '-');
        const afi = item.afi || '07';
        const securedLabel = t('afi.secured');

        return `
            <tr class="hover:bg-base-200/50 transition-colors border-b border-base-200/60" data-alarm-id="${item.id}">
                <td class="py-2.5 px-3.5 whitespace-nowrap font-mono font-medium text-base-content/90">
                    ${timeStr}
                </td>
                <td class="py-2.5 px-3.5 whitespace-nowrap">
                    <span class="badge badge-sm badge-outline font-semibold">${gate}</span>
                </td>
                <td class="py-2.5 px-3.5 whitespace-nowrap font-mono">
                    <div class="font-bold text-error flex items-center gap-1.5">
                        <span>${barcode}</span>
                    </div>
                    ${uid && uid !== barcode ? `<div class="text-[10px] text-base-content/60 font-mono">${uid}</div>` : ''}
                </td>
                <td class="py-2.5 px-3.5">
                    <div class="catalog-info-cell" data-catalog-barcode="${barcode}">
                        <span class="text-base-content/50 italic">${t('catalog.loading')}</span>
                    </div>
                </td>
                <td class="py-2.5 px-3.5 text-center whitespace-nowrap">
                    <span class="badge badge-sm badge-error text-error-content font-mono font-bold" title="${t('afi.securedDescription')}">
                        ${afi} (${securedLabel})
                    </span>
                </td>
            </tr>
        `;
    }).join('');

    tbody.innerHTML = rows;

    const navIndicator = document.getElementById('nav-gate-alarm-indicator');
    if (navIndicator) {
        navIndicator.classList.remove('hidden');
    }
}

export function handleGateAlarmEvent(data) {
    let payload = data;
    if (typeof data === 'string') {
        try {
            payload = JSON.parse(data);
        } catch (e) {
            console.error('Error parsing gate_alarm payload:', e);
            return;
        }
    }
    if (!payload) return;

    // Trigger acoustic alarm sound
    playAlarmSound();

    if (Array.isArray(payload.items) && payload.items.length > 0) {
        for (const item of payload.items) {
            if (!activeAlarms.some(a => a.id === item.id || (a.uid === item.uid && a.timeString === item.timeString))) {
                activeAlarms.unshift(item);
            }
        }
    } else if (payload.item) {
        if (!activeAlarms.some(a => a.id === payload.item.id)) {
            activeAlarms.unshift(payload.item);
        }
    }

    // Limit active alarms in memory
    if (activeAlarms.length > 100) {
        activeAlarms = activeAlarms.slice(0, 100);
    }

    // Display toast notification & update table
    showAlarmToast(payload);
    renderAlarmTable();
}

export async function clearGateAlarms() {
    try {
        await fetch(apiURL('gate-alarms/clear'), { method: 'POST' });
    } catch (e) {
        console.warn('Failed to clear gate alarms on server:', e);
    }
    activeAlarms = [];
    hideAlarmToast();
    renderAlarmTable();
}

export function initGateAlarm() {
    // Button listeners
    document.getElementById('gate-alarm-show-details-btn')?.addEventListener('click', openAlarmModal);
    document.getElementById('gate-alarm-dismiss-toast-btn')?.addEventListener('click', hideAlarmToast);
    document.getElementById('nav-gate-alarm-btn')?.addEventListener('click', openAlarmModal);
    document.getElementById('btn-clear-gate-alarms')?.addEventListener('click', clearGateAlarms);

    // Initial load of existing active alarms from backend
    fetch(apiURL('gate-alarms'))
        .then(res => res.json())
        .then(data => {
            if (data && Array.isArray(data.alarms) && data.alarms.length > 0) {
                activeAlarms = data.alarms;
                renderAlarmTable();
                const navIndicator = document.getElementById('nav-gate-alarm-indicator');
                if (navIndicator) {
                    navIndicator.classList.remove('hidden');
                }
            }
        })
        .catch(err => {
            console.debug('Could not load initial gate alarms:', err);
        });
}
