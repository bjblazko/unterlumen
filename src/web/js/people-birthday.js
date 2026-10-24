// people-birthday.js — a person's birthday (ADR-0051): kept by name in the
// folder installations share, and used to tell how old someone was in a
// photo. The person pages of People open the same dialog.

// The birthdays by name; empty when this installation keeps none.
async function peopleBirthdays() {
    try {
        return await PeopleAPI.birthdays();
    } catch {
        return {};
    }
}

// birthdayOf returns name's birthday ("2015-05-12") in any case, or ''.
function birthdayOf(birthdays, name) {
    const key = Object.keys(birthdays).find(k => k.toLowerCase() === name.toLowerCase());
    return key ? birthdays[key] : '';
}

// ageAt is how old someone born on birth ("2015-05-12") was on the day a
// date ("2020-06-01…") falls on, in whole years; null before birth.
function ageAt(birth, date) {
    if (!birth || !date || date.length < 10) return null;
    const [by, bm, bd] = birth.split('-').map(Number);
    const [y, m, d] = date.slice(0, 10).split('-').map(Number);
    const age = y - by - (m < bm || (m === bm && d < bd) ? 1 : 0);
    return age >= 0 ? age : null;
}

// ageText says an age with its unit: "8 years old", or "8 y" where there
// is little room (a chart's axis). A number alone is not an age.
function ageText(age, { short = false } = {}) {
    if (short) return `${age}\u00a0y`;
    return age === 0 ? 'under a year old' : `${age} ${age === 1 ? 'year' : 'years'} old`;
}

// The days on which someone born on birth was age years old.
function ageDates(birth, age) {
    const [y, m, d] = birth.split('-').map(Number);
    const from = new Date(Date.UTC(y + age, m - 1, d));
    const until = new Date(Date.UTC(y + age + 1, m - 1, d - 1));
    const iso = t => t.toISOString().slice(0, 10);
    return { date_taken_min: iso(from), date_taken_max: iso(until) };
}

// The line under a person's name: when they were born and how old they are.
function bornHTML(birth) {
    if (!birth) return '';
    const age = ageAt(birth, new Date().toISOString());
    return `<p class="people-born">${t('Born')} <span class="people-num">${escapeHtml(birth)}</span>${age !== null ? `, ${ageText(age)} today` : ''}</p>`;
}

// editBirthday asks for name's birthday and calls done after it was saved
// or taken away.
function editBirthday(name, current, done) {
    const actions = [{ label: t('Cancel'), value: null }];
    const save = async (d, value) => {
        try {
            await PeopleAPI.setBirthday(name, value);
            d.close(true);
            done();
        } catch (err) {
            const error = d.el.querySelector('.people-group-error');
            error.hidden = false;
            error.textContent = `${err.message} The birthday was not changed.`;
        }
    };
    if (current) actions.push({ label: t('Take it away'), onClick: (d) => save(d, '') });
    actions.push({ label: t('Save'), kind: 'accent', onClick: (d) => {
        const value = d.el.querySelector('#people-birthday').value;
        if (value) save(d, value);
    } });
    const dlg = new Dialog({
        title: t('Birthday'),
        subtitle: name,
        size: 'sm',
        className: 'library-dialog',
        body: `
            <label class="library-dialog-label" for="people-birthday">${t('Born on')}</label>
            <input class="library-dialog-input" id="people-birthday" type="date" max="${new Date().toISOString().slice(0, 10)}" value="${escapeHtml(current)}">
            <span class="form-hint">${t('Shows how old {name} was in each photo. Kept with your destinations in the folder your installations share, in every library; not written into the photos.', { name: escapeHtml(name) })}</span>
            <p class="people-group-error" hidden></p>`,
        actions,
    });
    dlg.open().querySelector('#people-birthday').focus();
}
