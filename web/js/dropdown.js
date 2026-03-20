// Shared dropdown toggle behaviour.
// Usage: Dropdown.init(triggerBtn, menuEl, { onOpen })
// Returns a { close } handle.

const Dropdown = {
    init(btn, menu, options = {}) {
        const close = () => {
            menu.style.display = 'none';
            btn.classList.remove('dropdown-open');
            document.removeEventListener('click', close);
        };

        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            if (btn.disabled) return;
            if (menu.style.display !== 'none') {
                close();
            } else {
                menu.style.display = '';
                btn.classList.add('dropdown-open');
                document.addEventListener('click', close);
                if (options.onOpen) options.onOpen();
            }
        });

        menu.addEventListener('click', (e) => e.stopPropagation());

        return { close };
    },
};
