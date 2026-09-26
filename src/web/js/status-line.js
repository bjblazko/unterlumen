// StatusLine — the foot of the sidebar shows the work the server is doing
// that outlives the page that started it: library scans, exports,
// publishing, site rebuilds, deploys (ADR-0036). The place that started the
// work shows it too; this is where it stays in view after you move on.

const STATUS_LINE_MAX_SHOWN = 3;
const STATUS_LINE_KEEP_DONE_MS = 8000;

// What a step of a publish run is doing, in words.
const PUBLISH_STEPS = {
    zip: 'Packing the download…',
    html: 'Writing the gallery…',
    site: 'Updating the site…',
};

class StatusLine {
    constructor(el) {
        this.el = el;
        this._jobs = new Map();     // id → job, as the server last described it
        this._views = new Map();    // id → { el, activity, title, mark }
        this._visible = new Set();  // ids that ran long enough to be shown
        this._timers = new Map();   // id → timeout (show after delay, hide after done)
        this._connect();
    }

    _connect() {
        if (typeof EventSource === 'undefined') return;
        // EventSource reconnects on its own; the server starts every
        // connection with all jobs it knows, so nothing is lost in between.
        this._source = new EventSource('/api/jobs/stream');
        this._source.onmessage = (e) => {
            try { this._update(JSON.parse(e.data)); } catch { /* malformed event */ }
        };
    }

    _update(job) {
        const known = this._jobs.get(job.id);
        this._jobs.set(job.id, job);
        if (!known) this._schedule(job);
        if (job.finished) this._settle(job);
        this._render();
    }

    // A job appears only once it has run for the activity delay, so a quick
    // rebuild never flickers through the sidebar. A failure always appears.
    _schedule(job) {
        if (job.finished) return;
        const age = Date.now() - new Date(job.started).getTime();
        const wait = Math.max(0, ACTIVITY_DELAY_MS - age);
        this._timers.set(job.id, setTimeout(() => {
            this._visible.add(job.id);
            this._render();
        }, wait));
    }

    // A finished job stays for a moment so its result can be read; a failed
    // one stays until it is clicked. One nobody saw running is not announced.
    _settle(job) {
        clearTimeout(this._timers.get(job.id));
        if (job.error) {
            this._visible.add(job.id);
            return;
        }
        if (!this._visible.has(job.id)) {
            this._forget(job.id);
            return;
        }
        const age = Date.now() - new Date(job.ended).getTime();
        this._timers.set(job.id, setTimeout(() => {
            this._forget(job.id);
            this._render();
        }, Math.max(0, STATUS_LINE_KEEP_DONE_MS - age)));
    }

    _forget(id) {
        clearTimeout(this._timers.get(id));
        this._timers.delete(id);
        this._visible.delete(id);
        this._jobs.delete(id);
        this._views.get(id)?.el.remove();
        this._views.delete(id);
    }

    _render() {
        const shown = [...this._jobs.values()].filter(j => this._visible.has(j.id));
        this.el.hidden = shown.length === 0;
        for (const [id, view] of this._views) {
            if (!shown.some(j => j.id === id)) { view.el.remove(); this._views.delete(id); }
        }
        shown.slice(0, STATUS_LINE_MAX_SHOWN).forEach(job => this._renderJob(job));
        this._renderMore(shown.length - STATUS_LINE_MAX_SHOWN);
    }

    _renderJob(job) {
        let view = this._views.get(job.id);
        if (!view) {
            view = this._createView(job);
            this._views.set(job.id, view);
        }
        this.el.insertBefore(view.el, this._moreEl || null);

        const state = job.error ? 'failed' : job.finished ? 'done' : 'running';
        view.el.dataset.state = state;
        const summary = this._describe(job, view.activity);
        view.el.title = `${job.title}: ${summary}`;
        view.el.setAttribute('aria-label', view.el.title);
    }

    _createView(job) {
        const el = document.createElement(job.place ? 'a' : 'div');
        el.className = 'status-job';
        if (job.place) {
            el.href = '#' + job.place;
            // Opening the place is reading the result; a failure has been seen.
            el.addEventListener('click', () => {
                const j = this._jobs.get(job.id);
                if (j?.finished) { this._forget(job.id); this._render(); }
            });
        }
        el.innerHTML = '<span class="status-job-mark" aria-hidden="true"></span><span class="status-job-title"></span>';
        el.querySelector('.status-job-title').textContent = job.title;
        const activity = new Activity({ delay: 0, since: job.started });
        el.appendChild(activity.el);
        return { el, activity };
    }

    // Puts the job's state into its activity line and returns it in words.
    _describe(job, activity) {
        if (job.error) {
            activity.fail(`It stopped: ${job.error}`);
            return `stopped: ${job.error}`;
        }
        if (job.finished) {
            const text = job.total > 0
                ? `Finished. ${formatCount(job.total)} ${this._noun(job)}.`
                : 'Finished.';
            activity.done(text);
            return text.toLowerCase();
        }
        if (job.total > 0) {
            activity.count(job.done, job.total, { noun: this._noun(job), current: job.current || '' });
            return `${job.done} of ${job.total} ${this._noun(job)}`;
        }
        const text = PUBLISH_STEPS[job.step] || (job.kind === 'library' ? 'Looking for photos…' : 'Working…');
        activity.busy(text);
        return text;
    }

    _noun(job) {
        return job.kind === 'library' || job.kind === 'export' || job.step === 'photo' ? 'photos' : 'items';
    }

    _renderMore(hidden) {
        if (hidden <= 0) { this._moreEl?.remove(); this._moreEl = null; return; }
        if (!this._moreEl) {
            this._moreEl = document.createElement('div');
            this._moreEl.className = 'status-more';
        }
        this.el.appendChild(this._moreEl);
        this._moreEl.textContent = `and ${hidden} more`;
    }
}
