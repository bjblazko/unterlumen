(function() {
    var p = localStorage.getItem('theme') || 'auto';
    var dark = p === 'dark' || (p === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
})();
