// Reusable toggle switch component

const Toggle = {
    create(container, options = {}) {
        const initial = options.initial || false;
        const onChange = options.onChange || null;
        const el = document.createElement('div');
        el.className = 'toggle';
        el.dataset.state = initial ? 'on' : 'off';
        el.setAttribute('role', 'switch');
        el.setAttribute('aria-checked', String(initial));
        el.innerHTML =
            '<span class="toggle-label toggle-label-on">ON</span>' +
            '<span class="toggle-track"><span class="toggle-thumb"></span></span>' +
            '<span class="toggle-label toggle-label-off">OFF</span>';
        el.addEventListener('click', () => {
            const on = el.dataset.state !== 'on';
            el.dataset.state = on ? 'on' : 'off';
            el.setAttribute('aria-checked', String(on));
            if (onChange) onChange(on);
        });
        if (container) container.appendChild(el);
        return {
            el,
            state: () => el.dataset.state === 'on',
            setState: (on) => {
                el.dataset.state = on ? 'on' : 'off';
                el.setAttribute('aria-checked', String(on));
            }
        };
    }
};
