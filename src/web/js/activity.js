// One way to say that something is happening (ADR-0036).
//
// Three levels, chosen by what is known:
//   busy(text)                      — words only; after 3 s the elapsed time follows
//   count(done, total, {noun, current}) — a bar and "412 of 1 280 photos"
//   done(text) / fail(text)         — what happened, in a sentence
//
// Nothing shows for the first 400 ms, so a quick answer never flickers. There is
// no looping animation: the only thing that moves is the bar and the seconds.

const ACTIVITY_DELAY_MS = 400;
const ACTIVITY_ELAPSED_AFTER_MS = 3000;

class Activity {
    // Replaces the content of `container` with an activity line. Returns the
    // Activity so the caller can move it through its states; replacing the
    // container's content later ends it.
    // `area: true` centres it in an otherwise empty content area.
    static in(container, text, { area = false } = {}) {
        container.innerHTML = '';
        const a = new Activity({ area });
        container.appendChild(a.el);
        if (text) a.busy(text);
        return a;
    }

    // Disables a button and puts the verb on it until restore() is called,
    // so one click is one request.
    static button(btn, verb) {
        const label = btn.textContent;
        btn.disabled = true;
        btn.setAttribute('aria-busy', 'true');
        if (verb) btn.textContent = verb;
        return () => {
            btn.disabled = false;
            btn.removeAttribute('aria-busy');
            if (verb) btn.textContent = label;
        };
    }

    // What went wrong, one line per item, at most five and a count of the rest.
    static errorList(lines) {
        const maxShow = 5;
        const shown = lines.slice(0, maxShow);
        if (lines.length > maxShow) shown.push(`and ${lines.length - maxShow} more`);
        const list = document.createElement('div');
        list.className = 'activity-errors';
        for (const l of shown) {
            const line = document.createElement('div');
            line.className = 'activity-error-line';
            line.textContent = l;
            list.appendChild(line);
        }
        return list;
    }

    // `delay` is how long to wait before showing anything; `since` is when
    // the work began, if that was before this line existed.
    constructor({ area = false, delay = ACTIVITY_DELAY_MS, since = null } = {}) {
        this._delay = delay;
        this._since = since ? new Date(since).getTime() : 0;
        this.el = document.createElement('div');
        this.el.className = 'activity' + (area ? ' activity--area' : '');
        this.el.setAttribute('role', 'status');
        this.el.setAttribute('aria-live', 'polite');
        this.el.hidden = true;
        this.el.innerHTML = `
            <div class="activity-line"><span class="activity-text"></span><span class="activity-elapsed"></span></div>
            <div class="activity-bar" role="progressbar" hidden><div class="activity-bar-fill"></div></div>
            <div class="activity-current"></div>`;
        this._text = this.el.querySelector('.activity-text');
        this._elapsed = this.el.querySelector('.activity-elapsed');
        this._bar = this.el.querySelector('.activity-bar');
        this._fill = this.el.querySelector('.activity-bar-fill');
        this._current = this.el.querySelector('.activity-current');
        this._started = 0;
        this._showTimer = null;
        this._tick = null;
    }

    busy(text) {
        this._begin();
        this.el.setAttribute('aria-live', 'polite');
        this.el.dataset.state = 'busy';
        this._text.textContent = text;
        this._bar.hidden = true;
        this._current.textContent = '';
        return this;
    }

    count(done, total, { noun = 'files', current = '' } = {}) {
        this._begin();
        // The bar carries the count for assistive technology; announcing
        // every file would drown out everything else.
        this.el.setAttribute('aria-live', 'off');
        this.el.dataset.state = 'busy';
        const d = Math.min(done, total);
        this._text.textContent = `${formatCount(d)} of ${formatCount(total)} ${noun}`;
        this._bar.hidden = false;
        this._bar.setAttribute('aria-valuemin', '0');
        this._bar.setAttribute('aria-valuemax', String(total));
        this._bar.setAttribute('aria-valuenow', String(d));
        this._fill.style.width = total > 0 ? (d / total * 100) + '%' : '0';
        this._current.textContent = current;
        return this;
    }

    done(text) { return this._finish('done', text); }
    fail(text) { return this._finish('failed', text); }

    clear() {
        this._stop();
        this.el.hidden = true;
        this.el.removeAttribute('data-state');
        this._text.textContent = '';
        this._elapsed.textContent = '';
        this._current.textContent = '';
        this._bar.hidden = true;
        this._setBusy(false);
        return this;
    }

    _begin() {
        if (this._started) return;
        this._started = this._since || Date.now();
        this._setBusy(true);
        if (this._delay > 0) this._showTimer = setTimeout(() => { this.el.hidden = false; }, this._delay);
        else this.el.hidden = false;
        this._showElapsed();
        this._tick = setInterval(() => this._onTick(), 1000);
    }

    // A line that was replaced along with its container stops counting.
    _onTick() {
        if (!this.el.isConnected) { this._stop(); return; }
        this._showElapsed();
    }

    _showElapsed() {
        const ms = Date.now() - this._started;
        this._elapsed.textContent = ms >= ACTIVITY_ELAPSED_AFTER_MS ? ` · ${Math.floor(ms / 1000)} s` : '';
    }

    _finish(state, text) {
        this._stop();
        this.el.setAttribute('aria-live', 'polite');
        this.el.hidden = false;
        this.el.dataset.state = state;
        this._text.textContent = text;
        this._elapsed.textContent = '';
        this._current.textContent = '';
        // Yellow means time passing; once it has passed, the sentence says
        // how it ended.
        this._bar.hidden = true;
        this._setBusy(false);
        return this;
    }

    _stop() {
        clearTimeout(this._showTimer);
        clearInterval(this._tick);
        this._showTimer = null;
        this._tick = null;
        this._started = 0;
    }

    _setBusy(on) {
        const host = this.el.parentElement;
        if (!host) return;
        if (on) host.setAttribute('aria-busy', 'true');
        else host.removeAttribute('aria-busy');
    }
}

// Thin space as thousands separator: "1 280", readable in mono and in any locale.
function formatCount(n) {
    return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}
