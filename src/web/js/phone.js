// A phone gets the layout for looking (ADR-0029's phone scope). Upright it
// is narrow; on its side it is wider than 700px but still low and touched,
// so it stays a phone instead of getting the desktop's sidebar and columns.
// The CSS repeats this query in every phone block, since a media query
// cannot be named there.

const PHONE_MEDIA = '(max-width: 700px), (max-height: 500px) and (pointer: coarse)';

function onPhone() {
    return matchMedia(PHONE_MEDIA).matches;
}
