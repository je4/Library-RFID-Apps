/**
 * Audio Feedback Module for RFID Desk
 * Manages Web Audio API oscillator with harmonic tones for scan, success, warning, and error events.
 */

import { t } from './i18n.js';

const STORAGE_KEY = 'rfid_audio_enabled';

let beepEnabled = false;
let audioCtx = null;
let lastBeepTime = 0;

function getStorage(name) {
    try {
        const val = localStorage.getItem(name);
        if (val !== null) return val;
    } catch (_) {}
    try {
        const match = document.cookie.match(new RegExp('(^|;\\s*)(' + name + ')=([^;]*)'));
        return match ? decodeURIComponent(match[3]) : null;
    } catch (_) {}
    return null;
}

function setStorage(name, value) {
    try {
        localStorage.setItem(name, value);
    } catch (_) {}
    try {
        document.cookie = `${name}=${encodeURIComponent(value)};path=/;max-age=31536000;SameSite=Lax`;
    } catch (_) {}
}

export function isAudioEnabled() {
    return beepEnabled;
}

export function setAudioEnabled(enabled, persist = true) {
    beepEnabled = Boolean(enabled);
    if (persist) {
        setStorage(STORAGE_KEY, beepEnabled ? 'true' : 'false');
    }
    const soundToggle = document.getElementById('sound-toggle');
    if (soundToggle) {
        soundToggle.classList.toggle('active', beepEnabled);
        soundToggle.setAttribute('aria-pressed', beepEnabled ? 'true' : 'false');
        soundToggle.setAttribute('title', t(beepEnabled ? 'audio.off' : 'audio.on'));
    }
    // Only eagerly resume/create audio context if triggered by user interaction (persist === true)
    if (beepEnabled && persist) {
        try {
            getAudioContext();
        } catch (e) {
            console.debug('AudioContext activation error:', e);
        }
    }
}

function getAudioContext() {
    if (!audioCtx) {
        const AudioContextClass = window.AudioContext || window.webkitAudioContext;
        if (AudioContextClass) {
            audioCtx = new AudioContextClass();
        }
    }
    if (audioCtx && audioCtx.state === 'suspended') {
        audioCtx.resume().catch(() => {});
    }
    return audioCtx;
}

/**
 * Standard scan chirp (880 Hz sine)
 */
export function playBeep() {
    if (!beepEnabled) return;
    const now = Date.now();
    if (now - lastBeepTime < 100) return;
    lastBeepTime = now;

    try {
        const ctx = getAudioContext();
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        osc.type = 'sine';
        const startTime = ctx.currentTime;
        osc.frequency.setValueAtTime(880, startTime);
        gain.gain.setValueAtTime(0.08, startTime);
        gain.gain.exponentialRampToValueAtTime(0.0001, startTime + 0.12);
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start(startTime);
        osc.stop(startTime + 0.12);
    } catch (e) {
        console.debug('Audio play error:', e);
    }
}

/**
 * Harmonic bright double-tone for successful completion (587Hz -> 880Hz)
 */
export function playSuccessSound() {
    if (!beepEnabled) return;
    try {
        const ctx = getAudioContext();
        const t0 = ctx.currentTime;
        
        // First tone (D5: 587 Hz)
        const osc1 = ctx.createOscillator();
        const gain1 = ctx.createGain();
        osc1.type = 'triangle';
        osc1.frequency.setValueAtTime(587.33, t0);
        gain1.gain.setValueAtTime(0.09, t0);
        gain1.gain.exponentialRampToValueAtTime(0.001, t0 + 0.12);
        osc1.connect(gain1);
        gain1.connect(ctx.destination);
        osc1.start(t0);
        osc1.stop(t0 + 0.12);

        // Second tone (A5: 880 Hz)
        const osc2 = ctx.createOscillator();
        const gain2 = ctx.createGain();
        osc2.type = 'sine';
        osc2.frequency.setValueAtTime(880, t0 + 0.1);
        gain2.gain.setValueAtTime(0.11, t0 + 0.1);
        gain2.gain.exponentialRampToValueAtTime(0.0001, t0 + 0.28);
        osc2.connect(gain2);
        gain2.connect(ctx.destination);
        osc2.start(t0 + 0.1);
        osc2.stop(t0 + 0.28);
    } catch (e) {
        console.debug('Success sound error:', e);
    }
}

/**
 * Warning alert tone (440Hz -> 330Hz)
 */
export function playWarningSound() {
    if (!beepEnabled) return;
    try {
        const ctx = getAudioContext();
        const t0 = ctx.currentTime;
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        osc.type = 'sawtooth';
        osc.frequency.setValueAtTime(440, t0);
        osc.frequency.linearRampToValueAtTime(330, t0 + 0.2);
        gain.gain.setValueAtTime(0.06, t0);
        gain.gain.exponentialRampToValueAtTime(0.0001, t0 + 0.22);
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start(t0);
        osc.stop(t0 + 0.22);
    } catch (e) {
        console.debug('Warning sound error:', e);
    }
}

/**
 * Deep double-tone for errors (220Hz / 180Hz)
 */
export function playErrorSound() {
    if (!beepEnabled) return;
    try {
        const ctx = getAudioContext();
        const t0 = ctx.currentTime;
        
        const osc1 = ctx.createOscillator();
        const gain1 = ctx.createGain();
        osc1.type = 'sawtooth';
        osc1.frequency.setValueAtTime(220, t0);
        gain1.gain.setValueAtTime(0.12, t0);
        gain1.gain.exponentialRampToValueAtTime(0.001, t0 + 0.14);
        osc1.connect(gain1);
        gain1.connect(ctx.destination);
        osc1.start(t0);
        osc1.stop(t0 + 0.14);

        const osc2 = ctx.createOscillator();
        const gain2 = ctx.createGain();
        osc2.type = 'sawtooth';
        osc2.frequency.setValueAtTime(175, t0 + 0.15);
        gain2.gain.setValueAtTime(0.12, t0 + 0.15);
        gain2.gain.exponentialRampToValueAtTime(0.0001, t0 + 0.35);
        osc2.connect(gain2);
        gain2.connect(ctx.destination);
        osc2.start(t0 + 0.15);
        osc2.stop(t0 + 0.35);
    } catch (e) {
        console.debug('Error sound error:', e);
    }
}

export function initAudio() {
    // Restore persistent sound preference across pages
    const saved = getStorage(STORAGE_KEY);
    if (saved === 'true') {
        setAudioEnabled(true, false);
    } else if (saved === 'false') {
        setAudioEnabled(false, false);
    }

    const soundToggle = document.getElementById('sound-toggle');
    if (soundToggle) {
        soundToggle.addEventListener('click', () => {
            setAudioEnabled(!beepEnabled, true);
        });
    }

    // Modern browsers require a user gesture before Web Audio API can output sound.
    // If audio is persisted as enabled, unlock the audio context on the first user interaction.
    const unlockAudio = () => {
        if (beepEnabled) {
            try {
                const ctx = getAudioContext();
                if (ctx && ctx.state === 'suspended') {
                    ctx.resume().catch(() => {});
                }
            } catch (_) {}
        }
        window.removeEventListener('pointerdown', unlockAudio);
        window.removeEventListener('keydown', unlockAudio);
        window.removeEventListener('click', unlockAudio);
    };
    window.addEventListener('pointerdown', unlockAudio, { once: true, passive: true });
    window.addEventListener('keydown', unlockAudio, { once: true, passive: true });
    window.addEventListener('click', unlockAudio, { once: true, passive: true });
}
