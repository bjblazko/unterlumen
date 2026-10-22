// place-header.js — the head of every place: its title and how many there
// are on the left, its actions and ⋯ on the right, what it is below. One
// head for all of them, so the same thing is always in the same spot.

// actions is HTML of framed buttons, the ⋯ last; a host that adds buttons
// later finds the slot as .place-header-actions. lede is a placeLede().
function placeHeaderHTML({ title, meta = '', actions = '', lede = '', below = '' }) {
    return `<header class="place-header">
        <div class="place-header-row">
            <h1 class="place-title">${title}</h1>
            <span class="place-meta">${meta}</span>
            <span class="place-header-actions">${actions}</span>
        </div>
        ${lede}${below}
    </header>`;
}
