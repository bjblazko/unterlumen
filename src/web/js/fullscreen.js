// Fullscreen — the browser's own full screen, without address bar or system
// bars, where the browser offers it (Safari on an iPhone allows it only for
// video, so there it is simply not offered). Whoever enters it leaves it:
// `enter` answers whether this call took the page into full screen, so a
// caller exits only what it entered, never a full screen the person chose.

const Fullscreen = {
    available() {
        return !!(document.fullscreenEnabled && document.documentElement.requestFullscreen);
    },

    active() {
        return !!document.fullscreenElement;
    },

    // Runs inside a tap or click, or the browser refuses it. The whole page
    // goes full screen, so dialogs and menus opened on top still show.
    async enter() {
        if (!this.available() || this.active()) return false;
        try {
            await document.documentElement.requestFullscreen({ navigationUI: 'hide' });
            return true;
        } catch {
            return false;
        }
    },

    exit() {
        if (this.active()) document.exitFullscreen().catch(() => {});
    },
};
