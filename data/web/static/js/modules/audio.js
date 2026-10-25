/**
 * Audio Feedback Module for RFID Desk
 * Manages Web Audio API oscillator and sound toggle button.
 */

let beepEnabled = false;
let audioCtx = null;
let lastBeepTime = 0;

export function isAudioEnabled() {
    return beepEnabled;
}

export function playBeep() {
    if (!beepEnabled) return;
    const now = Date.now();
    if (now - lastBeepTime < 100) return; // Prevent multiple overlapping bursts
    lastBeepTime = now;

    try {
        if (!audioCtx) {
            audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        }
        if (audioCtx.state === 'suspended') {
            audioCtx.resume();
        }
        const osc = audioCtx.createOscillator();
        const gain = audioCtx.createGain();
        osc.type = 'sine';
        const startTime = audioCtx.currentTime;
        osc.frequency.setValueAtTime(880, startTime); // 880 Hz
        gain.gain.setValueAtTime(0.08, startTime);
        gain.gain.exponentialRampToValueAtTime(0.0001, startTime + 0.12);
        osc.connect(gain);
        gain.connect(audioCtx.destination);
        osc.start(startTime);
        osc.stop(startTime + 0.12);
    } catch (e) {
        console.debug('Audio play error:', e);
    }
}

export function initAudio() {
    const soundToggle = document.getElementById('sound-toggle');
    if (soundToggle) {
        soundToggle.addEventListener('click', () => {
            beepEnabled = !beepEnabled;
            if (beepEnabled) {
                try {
                    if (!audioCtx) {
                        audioCtx = new (window.AudioContext || window.webkitAudioContext)();
                    }
                    if (audioCtx.state === 'suspended') {
                        audioCtx.resume();
                    }
                } catch (e) {
                    console.debug('AudioContext activation error:', e);
                }
            }
            soundToggle.classList.toggle('active', beepEnabled);
            soundToggle.setAttribute('aria-pressed', beepEnabled ? 'true' : 'false');
            soundToggle.setAttribute('title', beepEnabled ? 'Signalton ausschalten (Aktiv)' : 'Signalton einschalten (Inaktiv)');
        });
    }
}
